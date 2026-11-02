package app

import (
	"errors"
	"intentlens/internal/capture"
	"intentlens/internal/exercise"
	"os"
	"path/filepath"
	"strings"
)

func (a *App) scrubExercises() error {
	entries, err := os.ReadDir(a.State)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		id := strings.TrimSuffix(strings.TrimPrefix(name, "exercise-"), ".json")
		if !strings.HasPrefix(name, "exercise-") || !strings.HasSuffix(name, ".json") || !validID(id) {
			continue
		}
		var e Exercise
		if readErr := a.read(name, &e); readErr != nil {
			return readErr
		}
		revoked := e.SourceID != "" && !a.revisionExists(e.SourceID)
		paths := map[string]bool{}
		for p := range e.Spec.Baseline {
			paths[p] = true
		}
		paths[exercise.AcceptanceFile] = true
		answerDir := filepath.Join(a.State, "exercise-work-"+id, "answer")
		if resolved, err := filepath.EvalSymlinks(answerDir); err == nil && resolved == answerDir {
			answerEntries, err := os.ReadDir(answerDir)
			if err != nil {
				return err
			}
			for _, f := range answerEntries {
				if f.Name() != "go.mod" {
					paths[f.Name()] = true
				}
			}
		}
		var attempts []ExerciseAttempt
		if attemptsErr := a.read("exercise-attempts-"+id+".json", &attempts); attemptsErr != nil && !errors.Is(attemptsErr, os.ErrNotExist) {
			return attemptsErr
		}
		for i := range attempts {
			for p := range attempts[i].Files {
				paths[p] = true
			}
		}
		for p := range paths {
			ok, err := capture.Allowed(a.Root, exercise.JoinSource(e.PackageDir, p))
			if err != nil {
				return err
			}
			if !ok {
				revoked = true
			}
		}
		if !revoked {
			continue
		}
		dir := filepath.Join(a.State, "exercise-work-"+id)
		if info, err := os.Lstat(dir); err == nil {
			resolved, resolveErr := filepath.EvalSymlinks(dir)
			if resolveErr != nil || resolved != dir || !contained(a.State, dir) || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
				return errors.New("撤销练习目录含链接，请手动检查")
			}
			if removeErr := os.RemoveAll(dir); removeErr != nil {
				return removeErr
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		// Keep the principal record until every dependent removal succeeds, so failures retry.
		for _, prefix := range []string{"exercise-attempts-", "exercise-validation-", "exercise-auth-", "exercise-share-", "exercise-"} {
			if removeErr := os.Remove(filepath.Join(a.State, prefix+id+".json")); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removeErr
			}
		}
	}
	return nil
}

func (a *App) readExerciseAnswer(e Exercise, dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Name() == "go.mod" {
			continue
		}
		ok, allowedErr := capture.Allowed(a.Root, exercise.JoinSource(e.PackageDir, entry.Name()))
		if allowedErr != nil || !ok {
			return nil, errors.New("练习材料包含已排除路径，未读取内容")
		}
	}
	files, err := exercise.ReadWorkspace(dir, e.Spec)
	if err != nil {
		return nil, err
	}
	for name := range files {
		ok, err := capture.Allowed(a.Root, exercise.JoinSource(e.PackageDir, name))
		if err != nil || !ok {
			return nil, errors.New("练习材料包含已排除路径，未读取用于执行或模型")
		}
	}
	return files, nil
}
