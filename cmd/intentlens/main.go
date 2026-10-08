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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout)) }

type input struct {
	Session  string       `json:"session"`
	File     string       `json:"file"`
	Line     int          `json:"line"`
	Question string       `json:"question"`
	Settings app.Settings `json:"settings"`
	Consent  bool         `json:"consent"`
}

func run(args []string, in io.Reader, out io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(out, "IntentLens: start | stop | abort | rounds | index | inspect | explain | configure | status | hooks-config | hook")
		return 0
	}
	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	root := flags.String("root", ".", "project root")
	state := flags.String("state", "", "external local state directory")
	roundID := flags.String("round", "", "round id")
	if e := flags.Parse(args[1:]); e != nil {
		return fail(out, e)
	}
	if command == "hooks-config" {
		return emit(out, hookConfig())
	}
	if command == "hook" {
		handleHook(withoutBOM(in), *state)
		return 0
	}
	a, e := app.Open(*root, *state)
	if e != nil {
		return fail(out, e)
	}
	var request input
	// Read stdin only for commands that require it; never hang a background worker.
	switch command {
	case "start", "stop", "inspect", "explain", "configure":
		e = json.NewDecoder(io.LimitReader(withoutBOM(in), 32768)).Decode(&request)
		if e != nil {
			return fail(out, errors.New("需要有效的 JSON 输入"))
		}
	}
	var result any
	switch command {
	case "start":
		var active app.Active
		active, e = a.Start(request.Session)
		result = map[string]any{"id": active.ID, "started": active.Started}
	case "stop":
		result, e = a.Stop(request.Session)
	case "abort":
		e = a.Abort()
		result = map[string]bool{"aborted": e == nil}
	case "rounds":
		result, e = a.List()
	case "index":
		result, e = a.Index()
	case "inspect":
		result, e = a.Inspect(request.File, request.Line)
	case "explain":
		result, e = a.Explain(context.Background(), request.File, request.Line, request.Question, os.Getenv("ANTHROPIC_API_KEY"))
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
	case "analyze-round":
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

func emit(out io.Writer, v any) int {
	if json.NewEncoder(out).Encode(v) != nil {
		return 1
	}
	return 0
}

func withoutBOM(in io.Reader) io.Reader {
	reader := bufio.NewReader(in)
	if prefix, e := reader.Peek(3); e == nil && bytes.Equal(prefix, []byte{0xef, 0xbb, 0xbf}) {
		_, _ = reader.Discard(3)
	}
	return reader
}
func fail(out io.Writer, e error) int {
	_ = json.NewEncoder(out).Encode(map[string]string{"error": e.Error()})
	return 1
}

func hookConfig() any {
	// PATH command avoids shell-dependent quoting of local executable paths.
	return map[string]any{"hooks": map[string]any{
		"UserPromptSubmit": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "intentlens hook", "timeout": 10}}}},
		"Stop":             []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "intentlens hook", "timeout": 10}}}},
	}}
}

func handleHook(in io.Reader, state string) {
	var event struct {
		Event   string `json:"hook_event_name"`
		Session string `json:"session_id"`
		Root    string `json:"cwd"`
		Agent   string `json:"agent_id"`
	}
	if json.NewDecoder(io.LimitReader(in, 2*1024*1024)).Decode(&event) != nil {
		return
	}
	if event.Agent != "" || event.Root == "" {
		return
	}
	a, e := app.Open(event.Root, state)
	if e != nil {
		return
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
					cmd := exec.Command(executable, "analyze-round", "--root", a.Root, "--state", filepath.Dir(a.State), "--round", r.ID)
					hideWindow(cmd)
					if err = cmd.Start(); err == nil {
						_ = cmd.Process.Release()
					} else {
						e = errors.New("解释后台进程启动失败")
					}
				}
			}
		}
	default:
		return
	}
	if e != nil {
		b, _ := json.Marshal(map[string]string{"time": time.Now().UTC().Format(time.RFC3339), "event": event.Event, "error": e.Error()})
		_ = os.WriteFile(filepath.Join(a.State, "hook-diagnostic.json"), b, 0600)
	}
}

func analyzeRound(a *app.App, id string) error {
	rounds, e := a.List()
	if e != nil {
		return e
	}
	for _, r := range rounds {
		if r.ID != id {
			continue
		}
		idx, err := a.Index()
		if err != nil {
			return err
		}
		// One bounded request for the round; whole behavior grouping is generated by the model.
		_, err = a.ExplainRound(context.Background(), r, idx, os.Getenv("ANTHROPIC_API_KEY"))
		if err != nil {
			b, _ := json.Marshal(map[string]string{"round": id, "error": err.Error()})
			_ = os.WriteFile(filepath.Join(a.State, "analysis-diagnostic.json"), b, 0600)
		}
		return err
	}
	return errors.New("轮次不存在：" + strings.TrimSpace(id))
}
