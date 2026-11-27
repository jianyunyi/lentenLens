package capture

import "io/fs"

// capturedStat pairs the walk's FileInfo with a platform file identity
// recorded at walk time, so readCaptureFile can detect replacement.
type capturedStat struct {
	fs.FileInfo
	id fileIdentity
}
