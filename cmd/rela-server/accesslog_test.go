package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewAccessLogger_RejectsUnknownDestination(t *testing.T) {
	for _, dest := range []string{"", "file", "STDERR", "/var/log/x"} {
		if _, err := newAccessLogger(dest); err == nil {
			t.Errorf("newAccessLogger(%q) = nil error, want rejection", dest)
		}
	}
}

// Empty is "off" at flag level; otherwise only the two sinks are accepted.
func TestCheckAccessLogDest(t *testing.T) {
	for dest, ok := range map[string]bool{"": true, "stderr": true, "syslog": true, "file": false, "Syslog": false} {
		if err := checkAccessLogDest(dest); (err == nil) != ok {
			t.Errorf("checkAccessLogDest(%q) = %v, want ok=%v", dest, err, ok)
		}
	}
}

// Syslog stamps its own time and priority, so the message carries only the
// record: the line perf-report parses starts at msg=.
func TestSyslogAccessLogger_DropsTimeAndLevel(t *testing.T) {
	var buf bytes.Buffer
	newSyslogAccessLogger(&buf).Info("request", "method", "GET", "path", "/api/v1/x")
	got := strings.TrimSpace(buf.String())
	if want := "msg=request method=GET path=/api/v1/x"; got != want {
		t.Errorf("syslog line = %q, want %q", got, want)
	}
}
