package dataentry

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// stubReporter is a sandboxReporter with a fixed verdict, so both branches of
// the warning are exercised on every platform. Using a real CmdRunner would make
// the outcome depend on whether the test host has a working sandbox — which is
// how the branch that matters (an unusable sandbox, the systemd case this whole
// check exists for) would go untested on exactly the CI images that lack bwrap.
type stubReporter struct{ err error }

func (s stubReporter) SandboxErr() error { return s.err }

// captureWarn runs fn with the default logger swapped for one writing to a
// buffer, and returns what was logged.
func captureWarn(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)
	fn()
	return buf.String()
}

// scanWarnMeta builds a metamodel with one file property, optionally scanned.
func scanWarnMeta(t *testing.T, scanned bool) *metamodel.Metamodel {
	t.Helper()
	prop := metamodel.PropertyDef{Type: metamodel.PropertyTypeFile}
	if !scanned {
		prop.Scan = metamodel.ScanOff
	}
	return &metamodel.Metamodel{
		Attachments: &metamodel.AttachmentsConfig{ScanCmd: []string{"clamdscan", "{in}"}},
		Entities: map[string]metamodel.EntityDef{
			"thing": {Properties: map[string]metamodel.PropertyDef{"doc": prop}},
		},
	}
}

// TestWarnIfScanCannotRun covers the startup diagnostic that tells an operator
// their uploads are about to fail. Scanning is fail-closed, so a configured scan
// on a host that cannot run commands rejects EVERY upload to a scanned property
// — a state worth a warning at boot rather than a 422 on someone's first upload.
func TestWarnIfScanCannotRun(t *testing.T) {
	sandboxDown := errors.New("no working sandbox available")

	cases := []struct {
		name     string
		scanned  bool
		runner   sandboxReporter
		buildErr error
		wantWarn bool
		wantText string
	}{
		{
			name:     "scan configured, runner failed to build → warn",
			scanned:  true,
			runner:   nil,
			buildErr: errors.New("boom"),
			wantWarn: true,
			wantText: "could not be built",
		},
		{
			// The branch this whole ticket exists for: a hardened systemd unit
			// leaves the runner built but unable to confine anything.
			name:     "scan configured, sandbox unusable → warn",
			scanned:  true,
			runner:   stubReporter{err: sandboxDown},
			wantWarn: true,
			wantText: "no working sandbox",
		},
		{
			// Nothing is scanned, so a broken runner rejects nothing. Warning
			// here would be noise on every start of an unscanned project.
			name:     "scan off, runner failed to build → silent",
			scanned:  false,
			runner:   nil,
			buildErr: errors.New("boom"),
			wantWarn: false,
		},
		{
			name:     "scan off, sandbox unusable → silent",
			scanned:  false,
			runner:   stubReporter{err: sandboxDown},
			wantWarn: false,
		},
		{
			name:     "scan configured, sandbox healthy → silent",
			scanned:  true,
			runner:   stubReporter{},
			wantWarn: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			meta := scanWarnMeta(t, tc.scanned)
			out := captureWarn(t, func() {
				warnIfScanCannotRun(meta, tc.runner, tc.buildErr)
			})
			gotWarn := strings.Contains(out, "level=WARN")
			if gotWarn != tc.wantWarn {
				t.Fatalf("warned = %v, want %v (log: %q)", gotWarn, tc.wantWarn, out)
			}
			if tc.wantText != "" && !strings.Contains(out, tc.wantText) {
				t.Errorf("log %q does not mention %q", out, tc.wantText)
			}
		})
	}
}

// TestWarnIfScanCannotRun_NilMetamodel pins that the diagnostic cannot itself
// take the server down. A check whose purpose is to surface a degraded state is
// the worst possible place for a panic.
func TestWarnIfScanCannotRun_NilMetamodel(t *testing.T) {
	out := captureWarn(t, func() {
		warnIfScanCannotRun(nil, stubReporter{err: errors.New("down")}, nil)
	})
	if strings.Contains(out, "level=WARN") {
		t.Errorf("warned about a nil metamodel, which declares no scan: %q", out)
	}
}
