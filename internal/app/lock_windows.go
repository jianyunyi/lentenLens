package app

import (
	"os"
	"syscall"
	"unsafe"
)

var lockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("LockFileEx")
var unlockFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("UnlockFileEx")

func lockFile(f *os.File) error {
	var o syscall.Overlapped
	r, _, err := lockFileEx.Call(f.Fd(), 3, 0, 1, 0, uintptr(unsafe.Pointer(&o)))
	if r == 0 {
		return err
	}
	return nil
}
func unlockFile(f *os.File) {
	var o syscall.Overlapped
	_, _, _ = unlockFileEx.Call(f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&o)))
}

// Windows does not support the Unix directory fsync protocol. File Sync is
// still required, but sudden power loss is outside the process-crash guarantee.
func syncDir(*os.File) error { return nil }
