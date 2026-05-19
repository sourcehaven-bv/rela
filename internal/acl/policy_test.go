package acl_test

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// AC2.1: empty file decodes to a zero Policy.
func TestLoadPolicy_Empty(t *testing.T) {
	path := writeTempPolicy(t, "")
	p, err := acl.LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p == nil {
		t.Fatal("LoadPolicy returned nil Policy")
	}
	if len(p.Roles) != 0 || len(p.Assignments) != 0 || len(p.RoleRelations) != 0 {
		t.Errorf("expected zero Policy, got %+v", p)
	}
}

// AC2.1: a complete acl.yaml example round-trips into the typed shape
// downstream code consumes.
func TestLoadPolicy_FullExample(t *testing.T) {
	const yaml = `
user_entity_type: person
roles:
  admin:
    write: ["*"]
    read: ["*"]
    permissions: [delegate-admin, delegate-contributor]
  contributor:
    write: [ticket, concept]
    read: ["*"]
    permissions: [delegate-reviewer]
  reviewer:
    write: [review-response]
    read: ["*"]
  default:
    read: ["*"]
assignments:
  jeroen: admin
  alice:  contributor
  bob:    reviewer
role_relations:
  ticket-owner:
    confers: contributor
    requires_permission: delegate-contributor
`
	path := writeTempPolicy(t, yaml)
	p, err := acl.LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.UserEntityType != "person" {
		t.Errorf("UserEntityType = %q, want %q", p.UserEntityType, "person")
	}
	if len(p.Roles) != 4 {
		t.Errorf("len(Roles) = %d, want 4", len(p.Roles))
	}
	if got := p.Roles["admin"].Write; len(got) != 1 || got[0] != "*" {
		t.Errorf("admin.Write = %v, want [*]", got)
	}
	if got := p.Roles["contributor"].Permissions; len(got) != 1 || got[0] != "delegate-reviewer" {
		t.Errorf("contributor.Permissions = %v, want [delegate-reviewer]", got)
	}
	if p.Assignments["alice"] != "contributor" {
		t.Errorf("Assignments[alice] = %q, want contributor", p.Assignments["alice"])
	}
	rr := p.RoleRelations["ticket-owner"]
	if rr.Confers != "contributor" || rr.RequiresPermission != "delegate-contributor" {
		t.Errorf("RoleRelations[ticket-owner] = %+v, want {confers: contributor, requires_permission: delegate-contributor}", rr)
	}
}

// AC2.1: unknown top-level keys emit one slog.Warn per key and are
// otherwise ignored — the loader returns the typed Policy with known
// fields populated.
func TestLoadPolicy_UnknownKey_LogsWarning(t *testing.T) {
	const yaml = `
roles:
  admin:
    write: ["*"]
unknown_top_level: oops
also_unknown:
  nested: true
`
	path := writeTempPolicy(t, yaml)

	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	p, err := acl.LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.Roles["admin"].Write[0] != "*" {
		t.Errorf("known fields not populated: %+v", p)
	}

	logs := buf.String()
	for _, want := range []string{"unknown_top_level", "also_unknown"} {
		if !contains(logs, want) {
			t.Errorf("expected warning for %q, got logs:\n%s", want, logs)
		}
	}
}

// AC2.1: missing file returns an error wrapping os.ErrNotExist so
// callers (appbuild in PR 3) can fall back to NopACL via errors.Is.
func TestLoadPolicy_MissingFile_ReturnsErrNotExist(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	_, err := acl.LoadPolicy(missing)
	if err == nil {
		t.Fatal("LoadPolicy returned nil error for missing file")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err = %v, want errors.Is(_, os.ErrNotExist)", err)
	}
}

// AC2.1: malformed YAML returns a wrapped parse error (not a panic,
// not os.ErrNotExist).
func TestLoadPolicy_MalformedYAML_ReturnsParseError(t *testing.T) {
	path := writeTempPolicy(t, "roles:\n  admin:\n    write: [not-closed\n")
	_, err := acl.LoadPolicy(path)
	if err == nil {
		t.Fatal("LoadPolicy returned nil error for malformed YAML")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Errorf("err wrapped os.ErrNotExist, want a parse error")
	}
	if !contains(err.Error(), "parse") {
		t.Errorf("err = %v, want substring 'parse'", err)
	}
}

// writeTempPolicy writes the given YAML to a temp file and returns its
// path. Lifetime is scoped to the test.
func writeTempPolicy(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "acl.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write temp policy: %v", err)
	}
	return path
}
