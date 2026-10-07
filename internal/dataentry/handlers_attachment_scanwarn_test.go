package dataentry

import (
	"bytes"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/cmdexec"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
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

// TestWarnIfNoSandboxReadPaths pins when an upgraded host is told it needs a
// purpose's read paths: only on Linux, only when commands are confined, and
// only for a purpose that has commands configured and no paths listed.
func TestWarnIfNoSandboxReadPaths(t *testing.T) {
	const scanVar, transformVar = cmdexec.EnvSandboxScanReadPaths, cmdexec.EnvSandboxTransformReadPaths
	withTransform := &metamodel.Metamodel{Transforms: map[string]metamodel.TransformDef{"pdf": {}}}
	withAttachmentStep := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"thing": {Properties: map[string]metamodel.PropertyDef{"doc": {
			Type:      metamodel.PropertyTypeFile,
			Scan:      metamodel.ScanOff,
			Transform: []metamodel.TransformStep{{Cmd: []string{"qpdf", "{in}", "{out}"}}},
		}}},
	}}
	commandDoc := map[string]dataentryconfig.DocumentConfig{"report": {Command: []string{"pandoc", "{in}"}}}
	scanPaths := map[cmdexec.Purpose][]string{cmdexec.PurposeScan: {"/run/clamav/clamd.ctl"}}
	transformPaths := map[cmdexec.Purpose][]string{cmdexec.PurposeTransform: {"/etc/paperspecs"}}
	cases := []struct {
		name     string
		meta     *metamodel.Metamodel
		docs     map[string]dataentryconfig.DocumentConfig
		goos     string
		paths    map[cmdexec.Purpose][]string
		confined bool
		wantVars []string // the variables the warning names; none means quiet
	}{
		{"scan configured → warn scan", scanWarnMeta(t, true), nil, "linux", nil, true, []string{scanVar}},
		{"export transform → warn transform", withTransform, nil, "linux", nil, true, []string{transformVar}},
		{"attachment transform step → warn transform", withAttachmentStep, nil, "linux", nil, true,
			[]string{transformVar}},
		{"document command → warn transform", scanWarnMeta(t, false), commandDoc, "linux", nil, true,
			[]string{transformVar}},
		{"transform paths listed → quiet", withTransform, nil, "linux", transformPaths, true, nil},
		{"scan paths listed → quiet", scanWarnMeta(t, true), nil, "linux", scanPaths, true, nil},
		{"only the other purpose listed → warn", withTransform, nil, "linux", scanPaths, true,
			[]string{transformVar}},
		{"unconfined → quiet", withTransform, nil, "linux", nil, false, nil},
		{"macOS → quiet", withTransform, nil, "darwin", nil, true, nil},
		{"no commands → quiet", scanWarnMeta(t, false), nil, "linux", nil, true, nil},
		{"nil metamodel → quiet", nil, nil, "linux", nil, true, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			noReadPathsWarning = sync.Map{}
			got := captureWarn(t, func() {
				warnIfNoSandboxReadPaths(tc.meta, tc.docs, tc.goos, tc.paths, tc.confined)
			})
			for _, v := range []string{scanVar, transformVar} {
				if warned, want := strings.Contains(got, v), slices.Contains(tc.wantVars, v); warned != want {
					t.Errorf("warned about %s = %v, want %v; log: %q", v, warned, want, got)
				}
			}
		})
	}

	// Once per purpose per process: a multi-tenant server builds one App per tenant.
	noReadPathsWarning = sync.Map{}
	got := captureWarn(t, func() {
		warnIfNoSandboxReadPaths(withTransform, nil, "linux", nil, true)
		warnIfNoSandboxReadPaths(withTransform, nil, "linux", nil, true)
	})
	if n := strings.Count(got, transformVar); n != 1 {
		t.Errorf("warned %d times, want once; log: %q", n, got)
	}
}
