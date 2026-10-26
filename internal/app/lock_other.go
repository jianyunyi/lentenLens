//go:build !windows && !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package app

import (
	"errors"
	"os"
)

func lockFile(*os.File) error { return errors.New("此平台尚不支持项目操作锁") }
func unlockFile(*os.File)     {}
func syncDir(*os.File) error  { return errors.New("此平台尚不支持目录同步") }
