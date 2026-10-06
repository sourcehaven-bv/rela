//go:build !windows && !plan9 && !js && !wasip1

package main

import (
	"io"
	"log/syslog"
)

// dialSyslog connects to the local syslog socket (/dev/log), which
// journald owns on a systemd host.
func dialSyslog(tag string) (io.Writer, error) {
	return syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, tag)
}
