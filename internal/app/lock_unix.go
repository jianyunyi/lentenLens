//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package app

import (
	"os"
	"syscall"
)

func lockFile(f *os.File) error { return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) }
func unlockFile(f *os.File) {
	// The lock callback cannot return an error; a failed unlock must not trigger
	// a second syscall or hide the original operation result.
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_UN); err != nil {
		return
	}
}
func syncDir(f *os.File) error { return f.Sync() }
