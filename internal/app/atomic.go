package app

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// atomicReplace never truncates the target. Root.Rename confines both paths.
// before==nil is used only for private state records, not project source.
func atomicReplace(root *os.Root, name string, data []byte, mode os.FileMode, before *string) error {
	return atomicReplaceStaged(root, name, data, mode, before, "", nil)
}

func atomicReplaceStaged(root *os.Root, name string, data []byte, mode os.FileMode, before *string, stage string, beforeRename func() error) (err error) {
	if !filepath.IsLocal(name) {
		return errors.New("替换路径越界")
	}
	if info, err := root.Lstat(name); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("替换目标不是普通文件")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	dir := filepath.Dir(name)
	// Pin the parent directory too, so an ancestor swapped by another process
	// cannot redirect the temporary file or replacement to a different directory.
	parent, err := root.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer parent.Close()
	base := filepath.Base(name)
	temp := ".intentlens-write-" + newID()
	if stage != "" {
		if filepath.Dir(stage) != dir || !strings.HasPrefix(filepath.Base(stage), ".intentlens-write-") {
			return errors.New("暂存路径无效")
		}
		temp = filepath.Base(stage)
	}
	f, err := parent.OpenFile(temp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	defer func() {
		if removeErr := parent.Remove(temp); err == nil && !errors.Is(removeErr, os.ErrNotExist) {
			err = removeErr
		}
	}()
	if err = f.Chmod(mode.Perm()); err != nil {
		return errors.Join(err, f.Close())
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if before != nil {
		info, e := parent.Lstat(base)
		if e != nil || !info.Mode().IsRegular() {
			return errors.New("应用目标变化或包含链接")
		}
		current, e := parent.Open(base)
		if e != nil {
			return e
		}
		b, e := io.ReadAll(io.LimitReader(current, 60001))
		closeErr := current.Close()
		if e != nil || closeErr != nil {
			return errors.Join(e, closeErr)
		}
		if string(b) != *before {
			return errors.New("应用目标内容变化；未覆盖")
		}
	}
	if beforeRename != nil {
		if err = beforeRename(); err != nil {
			return err
		}
	}
	if err = parent.Rename(temp, base); err != nil {
		return err
	}
	d, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer d.Close()
	return syncDir(d)
}

func safeComponents(root *os.Root, name string) error {
	if !filepath.IsLocal(name) || strings.Contains(name, "\\") {
		return errors.New("恢复路径不安全")
	}
	path := ""
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == "" || part == "." || part == ".." {
			return errors.New("恢复路径不安全")
		}
		path = filepath.Join(path, part)
		i, e := root.Lstat(path)
		if e != nil {
			return e
		}
		if i.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("恢复路径包含链接")
		}
	}
	return nil
}
