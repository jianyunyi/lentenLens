package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"intentlens/internal/app"
	"intentlens/internal/explain"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout)) }

var openApp = app.Open

type input struct {
	ExerciseID          string                     `json:"exercise_id"`
	ParentID            string                     `json:"parent_id"`
	AttemptID           string                     `json:"attempt_id"`
	Mode                string                     `json:"mode"`
	ExplanationID       string                     `json:"explanation_id"`
	ClaimID             string                     `json:"claim_id"`
	Reason              string                     `json:"reason"`
	DraftID             string                     `json:"draft_id"`
	PreviewDigest       string                     `json:"preview_digest"`
	Confirm             bool                       `json:"confirm"`
	ApprovedEvidenceIDs []string                   `json:"approved_evidence_ids"`
	Session             string                     `json:"session"`
	Event               string                     `json:"event"`
	File                string                     `json:"file"`
	Line                int                        `json:"line"`
	Question            string                     `json:"question"`
	Code                string                     `json:"code"`
	Language            string                     `json:"language"`
	StartLine           int                        `json:"start_line"`
	EndLine             int                        `json:"end_line"`
	SelectionRange      explain.EditorRange        `json:"selection_range"`
	Diagnostics         []explain.EditorDiagnostic `json:"diagnostics"`
	SavedVersion        int                        `json:"saved_version"`
	Settings            app.Settings               `json:"settings"`
	Consent             bool                       `json:"consent"`
	RoundID             string                     `json:"round_id"`
}

func run(args []string, in io.Reader, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(out, "IntentLens (IDE / Skill / optional Claude): integration-info | capture-event | capture-status | capture-abort | start | stop | abort | rounds | index | inspect | call-path | explain | explain-selection | configure | status | test-model | analyze-round | hooks-config | hook")
		return 0
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "project root")
	state := flags.String("state", "", "external local state directory")
	roundID := flags.String("round", "", "round id")
	editorModelTarget := flags.String("editor-model-target", "", "VS Code credential's bound provider and base URL")
	if e := flags.Parse(args[1:]); e != nil {
		return fail(out, e)
	}
	if flags.NArg() != 0 {
		return fail(out, fmt.Errorf("不支持的位置参数：%q", flags.Arg(0)))
	}
	if command == "integration-info" {
		return emit(out, integrationInfo())
	}
	if command == "hooks-config" {
		return emit(out, hookConfig())
	}
	if command == "hook" {
		if e := handleHook(withoutBOM(in), *state); e != nil {
			return fail(out, e)
		}
		return 0
	}
	request, e := decodeRequest(command, in, roundID)
	if e != nil {
		return fail(out, e)
	}
	a, e := openApp(*root, *state)
	if e != nil {
		return fail(out, e)
	}
	if *editorModelTarget != "" {
		var target app.EditorModelTarget
		if err := json.Unmarshal([]byte(*editorModelTarget), &target); err != nil || target.Provider == "" || target.BaseURL == "" {
			return fail(out, errors.New("VS Code 模型密钥绑定无效，请重新保存 API Key"))
		}
		a.EditorModelTarget = &target
	}
	var result any
	switch command {
	case "capture-event":
		if strings.TrimSpace(request.Session) == "" {
			return fail(out, errors.New("通用事件需要非空 session；开始与结束须使用同一值"))
		}
		switch request.Event {
		case "start":
			var active app.Active
			active, e = a.Start(request.Session)
			result = map[string]any{"id": active.ID, "started": active.Started}
		case "stop":
			result, e = a.Stop(request.Session)
		default:
			return fail(out, errors.New("通用事件 event 必须为 start 或 stop"))
		}
	case "exercise-generate":
		var generated app.Exercise
		generated, e = a.GenerateExercise(context.Background(), request.ExplanationID, request.ClaimID, request.File, request.ParentID, "")
		if e == nil {
			result, e = a.ExerciseView(generated.ID)
		}
	case "exercises":
		result, e = a.Exercises()
	case "exercise-view":
		result, e = a.ExerciseView(request.ExerciseID)
	case "exercise-plan":
		result, e = a.PrepareExecution(request.ExerciseID)
	case "exercise-run":
		result, e = a.RunExercise(context.Background(), request.ExerciseID, request.PreviewDigest, request.Confirm, request.Mode)
	case "exercise-apply-preview":
		result, e = a.PrepareExerciseApply(request.ExerciseID)
	case "exercise-apply-confirm":
		e = a.ConfirmExerciseApply(request.ExerciseID, request.PreviewDigest, request.Confirm)
		result = map[string]bool{"applied": e == nil}
	case "exercise-share-prepare":
		result, e = a.PrepareExerciseShare(request.ExerciseID)
	case "exercise-share-confirm":
		e = a.ConfirmExerciseShare(request.ExerciseID, request.PreviewDigest, request.Confirm)
		result = map[string]bool{"shared": e == nil}
	case "exercise-shared":
		result, e = a.SharedExercises()
	case "exercise-import":
		result, e = a.ImportExercise(request.ExerciseID)
	case "exercise-feedback":
		result, e = a.ExerciseFeedback(context.Background(), request.ExerciseID, request.AttemptID, request.Reason, "")
	case "flag":
		result, e = a.FlagClaim(request.ExplanationID, request.ClaimID, request.Reason)
	case "adopt-shared":
		result, e = a.AdoptShared(request.ExplanationID)
	case "explanation":
		result, e = a.ExplanationByID(request.ExplanationID)
	case "revise":
		result, e = a.Revise(context.Background(), request.ExplanationID, "")
	case "share-prepare":
		result, e = a.PrepareShare(context.Background(), request.ExplanationID, request.ApprovedEvidenceIDs, "")
	case "share-confirm":
		result, e = a.ConfirmShare(request.DraftID, request.PreviewDigest, request.Confirm)
	case "shared":
		result, e = a.Shared()
	case "start":
		var active app.Active
		active, e = a.Start(request.Session)
		result = map[string]any{"id": active.ID, "started": active.Started}
	case "stop":
		result, e = a.Stop(request.Session)
	case "capture-status":
		result, e = a.CaptureState()
	case "capture-abort":
		e = a.AbortCapture(request.Session, request.RoundID, request.Confirm)
		result = map[string]bool{"aborted": e == nil}
	case "abort":
		e = a.Abort()
		result = map[string]bool{"aborted": e == nil}
	case "rounds":
		result, e = a.List()
	case "index":
		result, e = a.Index()
	case "inspect":
		result, e = a.Inspect(request.File, request.Line)
	case "call-path":
		result, e = a.CallPath(request.File, request.Line)
	case "explain":
		result, e = a.Explain(context.Background(), request.File, request.Line, request.Question, "")
	case "explain-selection":
		result, e = a.ExplainSelection(context.Background(), request.File, request.Code, request.Language, request.StartLine, request.EndLine, request.SavedVersion, request.SelectionRange, request.Diagnostics, "")
	case "configure":
		e = a.Configure(request.Settings, request.Consent)
		result = map[string]bool{"saved": e == nil}
	case "status":
		var s app.Settings
		s, e = a.Config()
		if e == nil {
			u, err := a.Usage()
			e = err
			diagnostics, diagnosticErr := a.Diagnostics()
			if e == nil {
				e = diagnosticErr
			}
			result = map[string]any{"settings": s, "usage": u, "state_directory": a.State, "root": a.Root, "diagnostics": diagnostics, "fees": "未配置费率，费用估算不可用"}
		}
	case "test-model":
		var tokens int
		tokens, e = a.TestModel(context.Background())
		result = map[string]any{"tested": e == nil, "actual_tokens": tokens, "fallback": false, "project_data_sent": false}
	case "analyze-round":
		if strings.TrimSpace(*roundID) == "" {
			return fail(out, errors.New("需要非空 round_id（可通过 --round 或 JSON 输入提供）"))
		}
		e = analyzeRound(a, *roundID)
		result = map[string]bool{"analyzed": e == nil}
	default:
		e = fmt.Errorf("未知命令：%s", command)
	}
	if e != nil {
		return fail(out, e)
	}
	return emit(out, result)
}

func decodeRequest(command string, in io.Reader, roundID *string) (input, error) {
	var request input
	// Read stdin only for commands that require it; never hang a background worker.
	switch command {
	case "capture-event", "capture-abort", "start", "stop", "inspect", "call-path", "explain", "explain-selection", "configure", "flag", "revise", "share-prepare", "share-confirm", "adopt-shared", "explanation", "exercise-generate", "exercise-view", "exercise-plan", "exercise-run", "exercise-apply-preview", "exercise-apply-confirm", "exercise-share-prepare", "exercise-share-confirm", "exercise-import", "exercise-feedback":
		limit := int64(32768)
		if command == "explain-selection" {
			limit = 128 * 1024
		}
		return decodeJSONRequest(in, limit)
	case "analyze-round":
		// VS Code's runCLI sends the round id as JSON. Background hook
		// workers pass --round explicitly and must not consume stdin.
		if strings.TrimSpace(*roundID) == "" {
			var err error
			request, err = decodeJSONRequest(in, 32768)
			if err != nil {
				return input{}, err
			}
			*roundID = request.RoundID
		}
	}
	return request, nil
}

func decodeJSONRequest(in io.Reader, limit int64) (input, error) {
	// Read one extra byte to distinguish real EOF from the size limit.
	reader := &io.LimitedReader{R: withoutBOM(in), N: limit + 1}
	decoder := json.NewDecoder(reader)
	var request *input
	var extra json.RawMessage
	if err := decoder.Decode(&request); err != nil || request == nil ||
		decoder.Decode(&extra) != io.EOF || reader.N == 0 {
		return input{}, errors.New("需要有效的 JSON 输入")
	}
	return *request, nil
}

func emit(out io.Writer, v any) int {
	if json.NewEncoder(out).Encode(v) != nil {
		return 1
	}
	return 0
}

func withoutBOM(in io.Reader) io.Reader {
	reader := bufio.NewReader(in)
	if prefix, e := reader.Peek(3); e == nil && bytes.Equal(prefix, []byte{0xef, 0xbb, 0xbf}) {
		if _, e = reader.Discard(3); e != nil {
			return reader
		}
	}
	return reader
}
func fail(out io.Writer, e error) int {
	if encodeErr := json.NewEncoder(out).Encode(map[string]string{"error": e.Error()}); encodeErr != nil {
		return 1
	}
	return 1
}

func hookConfig() any {
	// PATH command avoids shell-dependent quoting of local executable paths.
	return map[string]any{"hooks": map[string]any{
		"UserPromptSubmit": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "intentlens hook", "timeout": 10}}}},
		"Stop":             []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "intentlens hook", "timeout": 10}}}},
	}}
}

func handleHook(in io.Reader, state string) error {
	var event struct {
		Event   string `json:"hook_event_name"`
		Session string `json:"session_id"`
		Root    string `json:"cwd"`
		Agent   string `json:"agent_id"`
	}
	if json.NewDecoder(io.LimitReader(in, 2*1024*1024)).Decode(&event) != nil {
		return nil
	}
	if event.Agent != "" || event.Root == "" {
		return nil
	}
	a, e := app.Open(event.Root, state)
	if e != nil {
		return nil
	}
	switch event.Event {
	case "UserPromptSubmit":
		_, e = a.Start(event.Session)
	case "Stop":
		var r app.Round
		r, e = a.Stop(event.Session)
		if e == nil && len(r.Changes) > 0 {
			cfg, err := a.Config()
			if err == nil && cfg.Model.Enabled {
				executable, err := os.Executable()
				if err == nil {
					cmd := exec.Command(executable, "analyze-round", "--root", a.Root, "--state", filepath.Dir(a.State), "--round", r.ID) // #nosec G702 G204 -- runs this program directly with argv; exec.Command does not invoke a shell.
					hideWindow(cmd)
					if err = cmd.Start(); err == nil {
						if releaseErr := cmd.Process.Release(); releaseErr != nil {
							e = fmt.Errorf("解释后台进程句柄释放失败: %w", releaseErr)
						}
					} else {
						e = errors.New("解释后台进程启动失败")
					}
				}
			}
		}
	default:
		return nil
	}
	if e != nil {
		b, marshalErr := json.Marshal(map[string]string{"time": time.Now().UTC().Format(time.RFC3339), "event": event.Event, "error": e.Error()})
		if marshalErr != nil {
			return errors.Join(e, marshalErr)
		}
		if writeErr := writeStateDiagnostic(a.State, "hook-diagnostic.json", b); writeErr != nil {
			return errors.Join(e, writeErr)
		}
	}
	return nil
}

func analyzeRound(a *app.App, id string) error {
	rounds, e := a.List()
	if e != nil {
		return e
	}
	for i := range rounds {
		r := &rounds[i]
		if r.ID != id {
			continue
		}
		idx, err := a.Index()
		if err != nil {
			return err
		}
		// One bounded request for the round; whole behavior grouping is generated by the model.
		_, err = a.ExplainRound(context.Background(), *r, idx, "")
		if err != nil {
			b, marshalErr := json.Marshal(map[string]string{"round": id, "error": err.Error()})
			if marshalErr != nil {
				return errors.Join(err, marshalErr)
			}
			if writeErr := writeStateDiagnostic(a.State, "analysis-diagnostic.json", b); writeErr != nil {
				return errors.Join(err, writeErr)
			}
		} else {
			// A successful explicit retry supersedes the earlier failure for this
			// round, but must not erase a diagnostic belonging to another round.
			if diagnosticErr := removeRoundDiagnostic(a.State, id); diagnosticErr != nil {
				return diagnosticErr
			}
		}
		return err
	}
	return errors.New("轮次不存在：" + strings.TrimSpace(id))
}

func writeStateDiagnostic(state, name string, data []byte) error {
	if name != "hook-diagnostic.json" && name != "analysis-diagnostic.json" {
		return errors.New("invalid diagnostic file name")
	}
	root, err := os.OpenRoot(state)
	if err != nil {
		return err
	}
	writeErr := root.WriteFile(name, data, 0o600)
	return errors.Join(writeErr, root.Close())
}

func removeRoundDiagnostic(state, roundID string) error {
	root, err := os.OpenRoot(state)
	if err != nil {
		return err
	}
	data, readErr := root.ReadFile("analysis-diagnostic.json")
	if readErr != nil {
		closeErr := root.Close()
		if errors.Is(readErr, os.ErrNotExist) {
			return closeErr
		}
		return errors.Join(readErr, closeErr)
	}
	var diagnostic map[string]string
	var removeErr error
	if decodeErr := json.Unmarshal(data, &diagnostic); decodeErr == nil && diagnostic["round"] == roundID {
		removeErr = root.Remove("analysis-diagnostic.json")
		if errors.Is(removeErr, os.ErrNotExist) {
			removeErr = nil
		}
	}
	return errors.Join(removeErr, root.Close())
}
