//go:build windows

package capture

import (
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// fileIdentity is the volume serial number and file index of an open
// handle. Unlike a path-based lookup it cannot be redirected to a file
// that replaced the original.
type fileIdentity struct {
	vol, hi, lo uint32
	ok          bool
}

func handleIdentity(f *os.File) (fileIdentity, error) {
	var d syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &d); err != nil {
		return fileIdentity{}, err
	}
	return fileIdentity{vol: d.VolumeSerialNumber, hi: d.FileIndexHigh, lo: d.FileIndexLow, ok: true}, nil
}

// walkedIdentity records the identity of relative as seen at walk time.
// If the file cannot be opened or no longer matches walked, the identity
// is left unset and readCaptureFile fails closed with errFileChanged.
func walkedIdentity(root *os.Root, relative string, walked fs.FileInfo) capturedStat {
	s := capturedStat{FileInfo: walked}
	f, err := root.Open(filepath.FromSlash(relative))
	if err != nil {
		return s
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !sameFileVersion(walked, info) {
		return s
	}
	if id, err := handleIdentity(f); err == nil {
		s.id = id
	}
	return s
}

func sameIdentity(walked capturedStat, f *os.File) bool {
	if !walked.id.ok {
		return false
	}
	id, err := handleIdentity(f)
	return err == nil && id == walked.id
}
