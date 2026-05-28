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
  everyone:
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

// Affordance grants round-trip into the typed shape the resolver
// consumes: per-field write/visibility, per-option, per-relation
// with create/remove pointers and meta-field grants.
func TestLoadPolicy_AffordanceGrants(t *testing.T) {
	const yaml = `
roles:
  triager:
    write: [ticket]
    fields:
      ticket:
        - field: status
          when: "entity.assignee == current_user.id"
        - field: description
    visible:
      ticket:
        - field: internal_notes
          when: "has_global_role(current_user, 'admin')"
    options:
      ticket:
        - field: status
          option: done
          when: "has_role(current_user, entity, 'closer')"
    relations:
      ticket:
        - relation: implements
          create: true
          remove: false
          when: "entity.status == 'ready'"
        - relation: has-planning
          fields:
            - field: note
`
	path := writeTempPolicy(t, yaml)
	p, err := acl.LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	role := p.Roles["triager"]

	fields := role.Fields["ticket"]
	if len(fields) != 2 {
		t.Fatalf("fields[ticket] len = %d, want 2", len(fields))
	}
	if fields[0].Field != "status" || fields[0].When != "entity.assignee == current_user.id" {
		t.Errorf("fields[0] = %+v", fields[0])
	}
	if fields[1].Field != "description" || fields[1].When != "" {
		t.Errorf("fields[1] = %+v, want unconditional description", fields[1])
	}

	vis := role.Visible["ticket"]
	if len(vis) != 1 || vis[0].Field != "internal_notes" {
		t.Errorf("visible[ticket] = %+v", vis)
	}

	opts := role.Options["ticket"]
	if len(opts) != 1 || opts[0].Field != "status" || opts[0].Option != "done" {
		t.Errorf("options[ticket] = %+v", opts)
	}

	rels := role.Relations["ticket"]
	if len(rels) != 2 {
		t.Fatalf("relations[ticket] len = %d, want 2", len(rels))
	}
	if rels[0].Relation != "implements" {
		t.Errorf("relations[0].Relation = %q", rels[0].Relation)
	}
	if rels[0].Create == nil || !*rels[0].Create {
		t.Errorf("relations[0].Create = %v, want *true", rels[0].Create)
	}
	if rels[0].Remove == nil || *rels[0].Remove {
		t.Errorf("relations[0].Remove = %v, want *false", rels[0].Remove)
	}
	if rels[1].Relation != "has-planning" || rels[1].Create != nil || rels[1].Remove != nil {
		t.Errorf("relations[1] = %+v, want has-planning with nil Create/Remove", rels[1])
	}
	if len(rels[1].Fields) != 1 || rels[1].Fields[0].Field != "note" {
		t.Errorf("relations[1].Fields = %+v", rels[1].Fields)
	}

	if !p.HasAffordanceGrants() {
		t.Error("HasAffordanceGrants() = false, want true")
	}
}

// A policy carrying only write/read grants has no affordance blocks,
// so HasAffordanceGrants reports false and the entry point falls
// through to the permissive Nop resolver.
func TestPolicy_HasAffordanceGrants_NoneWhenWriteOnly(t *testing.T) {
	const yaml = `
roles:
  admin:
    write: ["*"]
    read: ["*"]
`
	p, err := acl.LoadPolicy(writeTempPolicy(t, yaml))
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.HasAffordanceGrants() {
		t.Error("HasAffordanceGrants() = true, want false for write-only policy")
	}
}

// The per-type affordance block's opt-in signal is the PRESENCE of
// the type key in the map, regardless of whether its value is an
// explicit empty list or a null. yaml.v3 decodes both `ticket: []`
// and `ticket:` (null) as a present key (nil slice for null), while
// an absent block leaves the key out of the map entirely. The
// resolver treats any present key as opt-in (closed-world deny-all
// when zero grants); only an absent key is permissive.
//
// We pin this here because the opt-in semantics hang on it (DR-C4):
// distinguishing nil-vs-empty-slice would be too subtle a hook for
// a security decision, so present-either-way = opt-in is the
// contract.
func TestLoadPolicy_AffordanceGrants_OptInIsKeyPresence(t *testing.T) {
	tests := []struct {
		name        string
		fieldsBlock string
		wantPresent bool
	}{
		{
			name:        "explicit empty list is opt-in",
			fieldsBlock: "    fields:\n      ticket: []\n",
			wantPresent: true,
		},
		{
			name:        "null value is opt-in (present key, nil slice)",
			fieldsBlock: "    fields:\n      ticket:\n",
			wantPresent: true,
		},
		{
			name:        "absent block is not opt-in",
			fieldsBlock: "",
			wantPresent: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := "roles:\n  triager:\n    write: [ticket]\n" + tc.fieldsBlock
			p, err := acl.LoadPolicy(writeTempPolicy(t, yaml))
			if err != nil {
				t.Fatalf("LoadPolicy: %v", err)
			}
			_, present := p.Roles["triager"].Fields["ticket"]
			if present != tc.wantPresent {
				t.Fatalf("ticket key present = %v, want %v (fields map = %#v)",
					present, tc.wantPresent, p.Roles["triager"].Fields)
			}
		})
	}
}

// Create/Remove *bool must distinguish explicit true, explicit
// false, and unset across the YAML forms operators actually write.
func TestLoadPolicy_RelationGrant_CreateRemovePointers(t *testing.T) {
	tests := []struct {
		name       string
		createLine string
		wantNil    bool
		wantVal    bool
	}{
		{name: "explicit true", createLine: "          create: true\n", wantNil: false, wantVal: true},
		{name: "explicit false", createLine: "          create: false\n", wantNil: false, wantVal: false},
		{name: "absent", createLine: "", wantNil: true},
		{name: "null value", createLine: "          create:\n", wantNil: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			yaml := "roles:\n  triager:\n    relations:\n      ticket:\n" +
				"        - relation: implements\n" + tc.createLine
			p, err := acl.LoadPolicy(writeTempPolicy(t, yaml))
			if err != nil {
				t.Fatalf("LoadPolicy: %v", err)
			}
			rels := p.Roles["triager"].Relations["ticket"]
			if len(rels) != 1 {
				t.Fatalf("relations len = %d, want 1", len(rels))
			}
			create := rels[0].Create
			if tc.wantNil != (create == nil) {
				t.Fatalf("Create nil = %v, want %v", create == nil, tc.wantNil)
			}
			if !tc.wantNil && *create != tc.wantVal {
				t.Errorf("*Create = %v, want %v", *create, tc.wantVal)
			}
		})
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
