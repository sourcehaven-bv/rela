//go:build windows || plan9 || js || wasip1

package main

import (
	"errors"
	"io"
)

func dialSyslog(string) (io.Writer, error) {
	return nil, errors.New("syslog is not available on this platform")
}
