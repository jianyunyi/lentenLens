package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// The inode is persistent: unlinking it can allow two independent lock owners.
// A legacy lock file is harmless once all pre-upgrade processes have stopped.
func (a *App) lock(name string) (func(), error) {
	if name != "capture" && name != "analysis" && name != "exercise" && name != "revision" && name != "settings" && name != "state" {
		return nil, errors.New("无效操作锁")
	}
	root, err := os.OpenRoot(a.State)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	path := name + ".lock"
	if info, e := root.Lstat(path); e == nil {
		if !info.Mode().IsRegular() {
			return nil, errors.New("操作锁必须是普通文件")
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	f, err := root.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	linkInfo, linkErr := os.Lstat(filepath.Join(a.State, path))
	if err != nil || linkErr != nil || !info.Mode().IsRegular() || !os.SameFile(info, linkInfo) {
		f.Close()
		return nil, errors.New("操作锁路径变化")
	}
	if err = lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("项目操作进行中（%s）: %w", name, err)
	}
	var once sync.Once
	return func() { once.Do(func() { unlockFile(f); _ = f.Close() }) }, nil
}
