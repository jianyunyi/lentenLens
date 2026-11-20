package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"intentlens/internal/explain"
	"io"
	"os"
	"strconv"
	"strings"
)

// Two 16 MiB text snapshots can expand to nearly 192 MiB when JSON-escaped.
const maxStateRecordBytes = 256 << 20

func recordTarget(name string) any {
	switch name {
	case "active.json":
		return &Active{}
	case "settings.json":
		return &Settings{}
	case "usage.json":
		return &explain.Usage{}
	case "hook-diagnostic.json", "analysis-diagnostic.json", "recovery-diagnostic.json":
		return &map[string]string{}
	}
	if !strings.HasSuffix(name, ".json") {
		return nil
	}
	switch {
	case strings.HasPrefix(name, "round-"):
		return &Round{}
	case strings.HasPrefix(name, "capture-ref-"):
		return new(string)
	case strings.HasPrefix(name, "explanation-"):
		return &Explanation{}
	case strings.HasPrefix(name, "correction-"):
		return &Correction{}
	case strings.HasPrefix(name, "share-draft-"):
		return &ShareDraft{}
	case strings.HasPrefix(name, "exercise-attempts-"):
		return &[]ExerciseAttempt{}
	case strings.HasPrefix(name, "exercise-validation-"):
		return &ExerciseValidation{}
	case strings.HasPrefix(name, "exercise-auth-"):
		return &ExecutionPlan{}
	case strings.HasPrefix(name, "exercise-share-"):
		return &ExerciseShareDraft{}
	case strings.HasPrefix(name, "exercise-"):
		return &Exercise{}
	}
	return nil
}

func (a *App) recoverState() error {
	// Use the same exercise -> capture order as execution/loadExercise.
	release, err := a.lock("exercise")
	if err != nil {
		return err
	}
	defer release()
	releaseCapture, err := a.lock("capture")
	if err != nil {
		return err
	}
	defer releaseCapture()
	if cleanupErr := a.cleanupStateTemps(); cleanupErr != nil {
		return cleanupErr
	}
	if isolateErr := a.isolateRecords(); isolateErr != nil {
		return isolateErr
	}
	if err := a.recoverActiveRound(); err != nil {
		return err
	}
	return a.recoverApply()
}

func (a *App) recoverActiveRound() error {
	var active Active
	if err := a.read("active.json", &active); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	// Leave malformed baseline metadata to the normal capture state checks.
	if !validID(active.ID) {
		return nil
	}
	if _, err := a.committedRound(active.Session, active.ID); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return a.finishRound(active)
}

func (a *App) isolateRecords() error {
	root, err := os.OpenRoot(a.State)
	if err != nil {
		return err
	}
	defer root.Close()
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return err
	}
	if err = root.Mkdir("quarantine", 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	info, err := root.Lstat("quarantine")
	if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return errors.New("隔离目录不安全")
	}
	q, err := root.OpenRoot("quarantine")
	if err != nil {
		return err
	}
	defer q.Close()
	old, err := q.ReadFile("policy")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if string(old) != policy.Hash() {
		dir, dirErr := q.Open(".")
		if dirErr != nil {
			return dirErr
		}
		entries, readErr := dir.ReadDir(-1)
		closeErr := dir.Close()
		err = errors.Join(readErr, closeErr)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.Name() != "policy" {
				if removeErr := q.Remove(e.Name()); removeErr != nil {
					return removeErr
				}
			}
		}
		if replaceErr := atomicReplace(q, "policy", []byte(policy.Hash()), 0o600, nil); replaceErr != nil {
			return replaceErr
		}
	}
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		target := recordTarget(entry.Name())
		if target == nil {
			continue
		}
		info, statErr := root.Lstat(entry.Name())
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() {
			return errors.New("本地记录不是普通文件")
		}
		// Historical records have a finite read bound. Never reset a corrupt budget
		// or consent file: those remain fail-closed until explicitly repaired.
		var parseErr error
		if info.Size() > maxStateRecordBytes {
			parseErr = errors.New("记录超过恢复范围")
		} else {
			f, e := root.Open(entry.Name())
			if e != nil {
				return e
			}
			b, e := io.ReadAll(io.LimitReader(f, (maxStateRecordBytes)+1))
			closeErr := f.Close()
			if e != nil || closeErr != nil {
				return errors.Join(e, closeErr)
			}
			if len(b) > maxStateRecordBytes {
				parseErr = errors.New("记录超过恢复范围")
			} else {
				parseErr = validateStateRecord(entry.Name(), b, target)
			}
			if parseErr == nil && string(bytes.TrimSpace(b)) == "null" {
				parseErr = errors.New("空记录")
			}
		}
		if parseErr == nil {
			continue
		}
		if entry.Name() == "settings.json" || entry.Name() == "usage.json" {
			return fmt.Errorf("%s 损坏；为保留授权和预算，需显式修复", entry.Name())
		}
		name := "quarantine/" + entry.Name() + "-" + newID() + ".bad"
		if renameErr := root.Rename(entry.Name(), name); renameErr != nil {
			return renameErr
		}
	}
	dir, dirErr := q.Open(".")
	if dirErr != nil {
		return dirErr
	}
	quarantined, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	err = errors.Join(readErr, closeErr)
	if err != nil {
		return err
	}
	names := []string{}
	for _, e := range quarantined {
		if strings.HasSuffix(e.Name(), ".bad") {
			names = append(names, e.Name())
		}
	}
	if len(names) > 0 {
		if diagnosticErr := a.write("recovery-diagnostic.json", map[string]string{"quarantined_count": strconv.Itoa(len(names)), "files": strings.Join(names, ", "), "action": "坏记录已隔离，不参与展示、共享或模型请求；排除规则变化时清除隔离原文。"}); diagnosticErr != nil {
			return diagnosticErr
		}
	} else if err = root.Remove("recovery-diagnostic.json"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Serializing with all state writers makes it safe to remove only abandoned
// private temporary files. No raw source from those files is read or surfaced.
func (a *App) cleanupStateTemps() error {
	release, err := a.lock("state")
	if err != nil {
		return err
	}
	defer release()
	root, err := os.OpenRoot(a.State)
	if err != nil {
		return err
	}
	defer root.Close()
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".intentlens-write-") {
			continue
		}
		info, statErr := root.Lstat(e.Name())
		if statErr != nil {
			return statErr
		}
		if !info.Mode().IsRegular() {
			return errors.New("私有暂存文件包含链接或目录；请人工检查")
		}
		if removeErr := root.Remove(e.Name()); removeErr != nil {
			return removeErr
		}
	}
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	return syncDir(f)
}

func validateStateRecord(name string, b []byte, target any) error {
	if err := json.Unmarshal(b, target); err != nil {
		return err
	}
	if string(bytes.TrimSpace(b)) == "null" {
		return errors.New("空记录")
	}
	if name != "usage.json" && name != "settings.json" {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	required := func(fields map[string]json.RawMessage, key string, v any) error {
		raw, ok := fields[key]
		if !ok || string(bytes.TrimSpace(raw)) == "null" {
			return errors.New("关键字段缺失")
		}
		return json.Unmarshal(raw, v)
	}
	if name == "usage.json" {
		var actual, uncertain int
		if err := required(fields, "actual_tokens", &actual); err != nil {
			return err
		}
		if err := required(fields, "uncertain_tokens", &uncertain); err != nil {
			return err
		}
		if actual < 0 || uncertain < 0 {
			return errors.New("用量无效")
		}
	} else {
		var model map[string]json.RawMessage
		if err := required(fields, "model", &model); err != nil {
			return err
		}
		var enabled bool
		var modelID string
		var limit int
		if err := required(model, "enabled", &enabled); err != nil {
			return err
		}
		if err := required(model, "model", &modelID); err != nil {
			return err
		}
		if err := required(model, "token_limit", &limit); err != nil {
			return err
		}
		if limit < 0 || (enabled && (limit == 0 || modelID == "")) {
			return errors.New("模型授权范围无效")
		}
		cfg, ok := target.(*Settings)
		if !ok {
			return errors.New("设置记录目标类型无效")
		}
		if _, err := cfg.Model.Resolved(); err != nil {
			return err
		}
	}
	return nil
}
