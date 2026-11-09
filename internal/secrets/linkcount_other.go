//go:build !unix && !windows

package secrets

import (
	"errors"
	"os"
)

// linkCount fails: this platform has no way to count hard links, so a
// declared credential file is refused rather than read unchecked.
func linkCount(*os.File) (uint64, error) {
	return 0, errors.New("this platform cannot count hard links")
}
