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
	return capturedStat{FileInfo: walked, id: fileIdentity{}}
}

func sameIdentity(walked capturedStat, _ *os.File) bool {
	// Off Windows there is no handle identity to compare: the caller's
	// os.SameFile check on the embedded FileInfo does the work.
	return walked.id == fileIdentity{}
}
