package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// accessLogTag is the syslog tag of --access-log=syslog. Under systemd it
// becomes the journal's SYSLOG_IDENTIFIER, so the access log is read with
// `journalctl -t rela-access` and the application log stays free of it.
const accessLogTag = "rela-access"

// checkAccessLogDest validates the --access-log flag value. Empty means off.
func checkAccessLogDest(dest string) error {
	switch dest {
	case "", "stderr", "syslog":
		return nil
	default:
		return fmt.Errorf("--access-log=%q: want stderr or syslog", dest)
	}
}

// newAccessLogger builds the logger behind --access-log for one of the two
// sinks; any other dest, including empty (off), is an error. Flag parsing
// validates with checkAccessLogDest and openAccessLog handles empty, so
// this is reached only with a real sink.
//
// The logger is separate from the default one on purpose: its records are
// Info regardless of --verbose/--quiet, and a dedicated sink keeps one line
// per request out of the application log.
func newAccessLogger(dest string) (*slog.Logger, error) {
	switch dest {
	case "stderr":
		return slog.New(slog.NewTextHandler(os.Stderr, nil)), nil
	case "syslog":
		w, err := dialSyslog(accessLogTag)
		if err != nil {
			return nil, fmt.Errorf("--access-log=syslog: %w", err)
		}
		return newSyslogAccessLogger(w), nil
	default:
		return nil, fmt.Errorf("--access-log=%q: no such sink", dest)
	}
}

// newSyslogAccessLogger drops time and level: syslog stamps its own
// timestamp and priority, so repeating them in the message is noise.
func newSyslogAccessLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && (a.Key == slog.TimeKey || a.Key == slog.LevelKey) {
				return slog.Attr{}
			}
			return a
		},
	}))
}

// openAccessLog opens the --access-log destination before the project is
// loaded, so an unusable one (no syslog socket in a sandbox, syslog on
// Windows) stops startup before any store is opened. Exit 1, not 2: the
// flag was valid, the environment is not.
//
// Nil: returned when dest is empty — the access log is off, and
// App.SetAccessLog treats nil as off.
func openAccessLog(dest string) *slog.Logger {
	if dest == "" {
		return nil
	}
	l, err := newAccessLogger(dest)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rela-server:", err)
		os.Exit(1)
	}
	return l
}
