//go:build !unix

package httpremote

import "io/fs"

// linkCount reports no count: the file info does not carry one here.
func linkCount(fs.FileInfo) (uint64, bool) { return 0, false }
