//go:build !windows

package capture

import (
	"io/fs"
	"os"
)

// fileIdentity is empty off Windows: os.SameFile already compares the
// device and inode captured by the walk's stat.
type fileIdentity struct{}

func walkedIdentity(_ *os.Root, _ string, walked fs.FileInfo) capturedStat {
	return capturedStat{FileInfo: walked}
}

func sameIdentity(capturedStat, *os.File) bool {
	return true
}
