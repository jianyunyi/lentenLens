package app

import (
	"errors"
	"fmt"
	"intentlens/internal/capture"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const applyJournalName = "apply-journal.json"

type applyJournal struct {
	ID         string           `json:"id"`
	Schema     int              `json:"schema"`
	PolicyHash string           `json:"policy_hash"`
	Changes    []ExerciseChange `json:"changes"`
}

func currentSource(root *os.Root, path string) (string, os.FileMode, error) {
	if err := safeComponents(root, path); err != nil {
		return "", 0, err
	}
	info, err := root.Lstat(path)
	if err != nil {
		return "", 0, err
	}
	if !info.Mode().IsRegular() || info.Size() > 60000 {
		return "", 0, errors.New("目标不是可应用的普通源码文件")
	}
	f, err := root.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 60001))
	if err != nil {
		return "", 0, err
	}
	if len(b) > 60000 {
		return "", 0, errors.New("目标文件超过应用范围")
	}
	return string(b), info.Mode().Perm(), nil
}

func (a *App) removeApplyJournal() error {
	root, err := os.OpenRoot(a.State)
	if err != nil {
		return err
	}
	defer root.Close()
	if err = root.Remove(applyJournalName); err != nil {
		return err
	}
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	return syncDir(f)
}

// recoverApply is called under the exercise lock. The journal contains complete
// backups, so no per-file progress counter can become stale after a crash.
func (a *App) recoverApply() error {
	var j applyJournal
	if err := a.read(applyJournalName, &j); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return errors.New("应用恢复日志损坏；保留日志并停止写入，请人工检查")
	}
	if !validID(j.ID) || j.Schema != 1 || len(j.Changes) == 0 || len(j.Changes) > 20 {
		return errors.New("应用恢复日志无效")
	}
	policy, err := capture.LoadPolicy(a.Root)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(a.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	seen := map[string]bool{}
	total := 0
	for _, c := range j.Changes {
		if !safeRelative(c.File) || !strings.HasSuffix(c.File, ".go") || seen[c.File] || c.Before == c.After {
			return errors.New("应用恢复路径或版本无效")
		}
		seen[c.File] = true
		total += len(c.Before) + len(c.After)
		if total > 120000 {
			return errors.New("应用恢复日志超出范围")
		}
	}
	if err = j.cleanupStages(root); err != nil {
		return err
	}
	if policy.Hash() != j.PolicyHash {
		return errors.New("应用恢复期间排除规则变化；暂存已清除，保留日志，请人工检查")
	}
	for _, c := range j.Changes {
		allowed, e := capture.Allowed(a.Root, c.File)
		if e != nil || !allowed {
			return errors.New("应用恢复路径已排除或不安全")
		}
		current, _, e := currentSource(root, c.File)
		if e != nil || (current != c.Before && current != c.After) {
			return errors.New("应用恢复遇到外部修改；已保留文件和恢复日志，请人工检查")
		}
	}
	for i, c := range j.Changes {
		current, mode, e := currentSource(root, c.File)
		if e != nil {
			return e
		}
		if current == c.Before {
			continue
		}
		if current != c.After {
			return errors.New("恢复期间目标变化，未覆盖外部修改")
		}
		if e = atomicReplaceStaged(root, c.File, []byte(c.Before), mode, &c.After, j.stage(i, "undo"), nil); e != nil {
			return fmt.Errorf("恢复未完成，保留日志: %w", e)
		}
	}
	if err = j.cleanupStages(root); err != nil {
		return err
	}
	return a.removeApplyJournal()
}

func (a *App) applyChanges(plan ExerciseApply) error {
	if len(plan.Changes) > 1 {
		return errors.New("多文件应用已关闭；一次仅允许单个源码文件，未改动任何文件")
	}
	if len(plan.Changes) == 0 {
		return errors.New("没有可应用的源码修改")
	}
	if err := a.recoverApply(); err != nil {
		return err
	}
	root, err := os.OpenRoot(a.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	// Validate every destination before journaling or changing any source.
	for _, c := range plan.Changes {
		current, _, e := currentSource(root, c.File)
		if e != nil || current != c.Before {
			return errors.New("应用前目标变化；没有写入任何文件")
		}
	}
	j := applyJournal{ID: newID(), Schema: 1, PolicyHash: plan.PolicyHash, Changes: plan.Changes}
	if err = a.write(applyJournalName, j); err != nil {
		return err
	}
	fail := func(cause error) error {
		if recoveryErr := a.recoverApply(); recoveryErr != nil {
			return fmt.Errorf("应用失败，恢复未完成，请保留日志: %v; %w", recoveryErr, cause)
		}
		return fmt.Errorf("应用失败，已恢复原内容: %w", cause)
	}
	for i, c := range plan.Changes {
		policy, e := capture.LoadPolicy(a.Root)
		if e != nil || policy.Hash() != plan.PolicyHash {
			return fail(errors.New("应用期间排除规则变化"))
		}
		_, mode, e := currentSource(root, c.File)
		if e != nil {
			return fail(e)
		}
		if e = atomicReplaceStaged(root, c.File, []byte(c.After), mode, &c.Before, j.stage(i, "apply"), a.applyStageFault); e != nil {
			return fail(e)
		}
		if a.applyFault != nil {
			if e = a.applyFault(i); e != nil {
				return fail(e)
			}
		}
	}
	if err = j.cleanupStages(root); err != nil {
		return err
	}
	if err = a.removeApplyJournal(); err != nil {
		return fmt.Errorf("源码已替换但日志清理失败；请保留日志重试恢复: %w", err)
	}
	return nil
}

// Every source temporary path is derivable from the persisted transaction ID.
func (j applyJournal) stage(i int, phase string) string {
	return filepath.Join(filepath.Dir(j.Changes[i].File), fmt.Sprintf(".intentlens-write-%s-%d-%s", j.ID, i, phase))
}
func (j applyJournal) cleanupStages(root *os.Root) error {
	for i, c := range j.Changes {
		dir := filepath.Dir(c.File)
		if dir != "." {
			if err := safeComponents(root, filepath.ToSlash(dir)); err != nil {
				return err
			}
		}
		for _, phase := range []string{"apply", "undo"} {
			name := j.stage(i, phase)
			if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		f, err := root.Open(dir)
		if err != nil {
			return err
		}
		err = syncDir(f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
