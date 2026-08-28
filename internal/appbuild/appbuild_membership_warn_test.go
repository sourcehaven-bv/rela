package appbuild_test

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

// ungatedWarningFragment is the load-bearing fragment of the membership
// warning asserted by BOTH the fires-test and the stays-quiet tests. Keep
// them on one constant: if they keyed off different substrings, rewording
// the message could break the positive test while silently turning the
// negative tests into vacuous passes.
const ungatedWarningFragment = "membership relation is NOT gated"

// captureWarnings redirects the default slog logger into a buffer for the
// duration of the test and returns an accessor for what was logged.
//
// Callers must NOT use t.Parallel(): slog.SetDefault mutates process-global
// state, so two parallel service builds would race on the logger and bleed
// log lines across tests — which would let a quiet-case assertion pass (or
// fail) on another test's output. Writes and reads are mutex-serialized
// because appbuild starts background goroutines (store watcher) that can
// log after New returns; bytes.Buffer is not safe for concurrent use.
func captureWarnings(t *testing.T) func() string {
	t.Helper()
	var mu sync.Mutex
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(
		&lockedWriter{mu: &mu, w: &buf}, &slog.HandlerOptions{Level: slog.LevelWarn})))
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		return buf.String()
	}
}

// lockedWriter serializes writes to the underlying writer with the same
// mutex captureWarnings reads under.
type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// TKT-T31NKT: a policy whose assignments confer a privileged role while the
// membership relation carries no requires_permission gate must warn loudly at
// startup — the server still boots (the hole is pre-existing and acceptable
// inside a trusted team), but the operator is told, without having to
// remember to run `rela acl audit`.
func TestBuildACL_UngatedMembership_WarnsAtStartup(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)
	writePolicy(t, root, `roles:
  admin:
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    read: ["*"]
assignments:
  engineering: admin
`)

	logs := captureWarnings(t)

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	got := logs()
	for _, want := range []string{ungatedWarningFragment, "member-of", "requires_permission", "docs/acl-security.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("startup warning missing %q; logs:\n%s", want, got)
		}
	}
}

// The warning must stay quiet for the shapes that are not an escalation path,
// so it does not become noise operators learn to ignore. Each case here is a
// policy that boots clean.
func TestBuildACL_MembershipWarning_QuietWhenSafe(t *testing.T) {
	tests := []struct {
		name   string
		policy string
	}{
		{
			name: "gated by requires_permission",
			policy: `roles:
  admin:
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    read: ["*"]
    permissions: [delegate-membership]
assignments:
  engineering: admin
role_relations:
  member-of:
    requires_permission: delegate-membership
`,
		},
		{
			// RR-EG5D3E: a read-only group is a visibility choice.
			name: "assigned role is read-only",
			policy: `roles:
  reader:
    read: ["*"]
assignments:
  engineering: reader
`,
		},
		{
			name: "no assignments at all",
			policy: `roles:
  admin:
    create: ["*"]
    update: ["*"]
    delete: ["*"]
    read: ["*"]
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeMetamodel(t, root)
			writePolicy(t, root, tc.policy)

			logs := captureWarnings(t)

			svc, err := appbuildOnDisk(t, root)
			if err != nil {
				t.Fatalf("appbuild.New: %v", err)
			}
			defer svc.Close()

			if got := logs(); strings.Contains(got, ungatedWarningFragment) {
				t.Errorf("expected no membership warning, got logs:\n%s", got)
			}
		})
	}
}

// A project with no acl.yaml falls back to NopACL; there is no policy to
// inspect, so nothing may be warned about.
func TestBuildACL_NoPolicy_NoMembershipWarning(t *testing.T) {
	root := t.TempDir()
	writeMetamodel(t, root)

	logs := captureWarnings(t)

	svc, err := appbuildOnDisk(t, root)
	if err != nil {
		t.Fatalf("appbuild.New: %v", err)
	}
	defer svc.Close()

	if got := logs(); strings.Contains(got, ungatedWarningFragment) {
		t.Errorf("expected no membership warning without acl.yaml, got logs:\n%s", got)
	}
}
