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
	t.Parallel()
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
	t.Parallel()
	const yaml = `
user_entity_type: person
roles:
  admin:
    create: ["*"]
    read: ["*"]
    permissions: [delegate-admin, delegate-contributor]
  contributor:
    create: [ticket, concept]
    read: ["*"]
    permissions: [delegate-reviewer]
  reviewer:
    create: [review-response]
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
	if got := p.Roles["admin"].Create; len(got) != 1 || got[0] != "*" {
		t.Errorf("admin.Create = %v, want [*]", got)
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
// NOT parallel: swaps the process-global default slog logger
// (TKT-VRZVXW — same class as t.Setenv; cannot run alongside
// parallel siblings).
func TestLoadPolicy_UnknownKey_LogsWarning(t *testing.T) {
	const yaml = `
roles:
  admin:
    create: ["*"]
    read: ["*"]
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
	if p.Roles["admin"].Create[0] != "*" {
		t.Errorf("known fields not populated: %+v", p)
	}

	logs := buf.String()
	for _, want := range []string{"unknown_top_level", "also_unknown"} {
		if !contains(logs, want) {
			t.Errorf("expected warning for %q, got logs:\n%s", want, logs)
		}
	}
}

// TKT-Z8A62F AC5: a non-default membership_relation that is not gated by
// a requires_permission role-relation is an escalation foot-gun; Validate
// emits an advisory slog.Warn and still succeeds (warn-and-continue).
// NOT parallel: swaps the process-global default slog logger.
func TestPolicy_MembershipRelation_UngatedWarns(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	// heeft_rol is configured + assigned but NOT gated by role_relations,
	// so the un-gated-escalation warning must fire. Validate must still
	// return a usable policy (no error).
	p, err := acl.LoadPolicyBytes([]byte(`
membership_relation: heeft_rol
assignments:
  engineering: editor
roles:
  editor:
    read: [ticket]
`))
	if err != nil {
		t.Fatalf("LoadPolicyBytes returned error; hardening checks must warn, not fail: %v", err)
	}
	if p.MembershipRelation != "heeft_rol" {
		t.Errorf("MembershipRelation = %q, want heeft_rol", p.MembershipRelation)
	}
	logs := buf.String()
	if !contains(logs, "requires_permission") || !contains(logs, "heeft_rol") {
		t.Errorf("expected un-gated membership_relation warning mentioning requires_permission + heeft_rol, got:\n%s", logs)
	}
}

// TKT-Z8A62F AC5 (companion): when the membership_relation IS gated by a
// role_relations.requires_permission entry, the escalation warning must
// NOT fire. The default member-of path is likewise silent (covered by the
// many existing tests that load default policies without warnings).
func TestPolicy_MembershipRelation_GatedNoWarn(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	if _, err := acl.LoadPolicyBytes([]byte(`
membership_relation: heeft_rol
assignments:
  engineering: editor
role_relations:
  heeft_rol:
    requires_permission: delegate-membership
roles:
  editor:
    read: [ticket]
    permissions: [delegate-membership]
`)); err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	if logs := buf.String(); contains(logs, "requires_permission") {
		t.Errorf("gated membership_relation must not warn, got:\n%s", logs)
	}
}

// AC2.1: missing file returns an error wrapping os.ErrNotExist so
// callers (appbuild in PR 3) can fall back to NopACL via errors.Is.
func TestLoadPolicy_MissingFile_ReturnsErrNotExist(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	path := writeTempPolicy(t, "roles:\n  admin:\n    create: [not-closed\n")
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
	t.Parallel()
	const yaml = `
roles:
  triager:
    create: [ticket]
    read: [ticket]
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
	t.Parallel()
	const yaml = `
roles:
  admin:
    create: ["*"]
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
	t.Parallel()
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
			t.Parallel()
			yaml := "roles:\n  triager:\n    create: [ticket]\n    read: [ticket]\n" + tc.fieldsBlock
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
	t.Parallel()
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
			t.Parallel()
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

// RR-NIGK: a blank entry in inherit_roles_through is rejected at load.
// A blank entry would otherwise reach StoreGraph with an empty
// RelationQuery.Type meaning "all relations" — silently widening
// containment to every relation type in the workspace.
func TestLoadPolicy_BlankInheritRolesThrough_Rejected(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		yaml string
	}{
		{"empty string", "inherit_roles_through:\n  - \"\"\n"},
		{"whitespace", "inherit_roles_through:\n  - \"  \"\n"},
		{"empty among valid", "inherit_roles_through:\n  - belongs-to\n  - \"\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempPolicy(t, tc.yaml)
			_, err := acl.LoadPolicy(path)
			if err == nil {
				t.Fatalf("LoadPolicy: expected error on blank inherit_roles_through entry; got nil")
			}
		})
	}
}

// RR-NIGK / RR-ZB1V: a blank key in role_relations is rejected —
// would otherwise gate every relation write on the delegate
// permission, breaking writes the operator didn't mean to gate.
// Covers both the empty-string and whitespace-only cases, mirroring
// the inherit_roles_through test's coverage.
func TestLoadPolicy_BlankRoleRelationsKey_Rejected(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		yaml string
	}{
		{"empty string", "role_relations:\n  \"\":\n    confers: contributor\n"},
		{"whitespace", "role_relations:\n  \"  \":\n    confers: contributor\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeTempPolicy(t, tc.yaml)
			_, err := acl.LoadPolicy(path)
			if err == nil {
				t.Fatalf("LoadPolicy: expected error on blank role_relations key; got nil")
			}
		})
	}
}

// TKT-4LQMWP (was TKT-VMD8 AC8 / RR-W2J6): a role granting UPDATE or DELETE on
// a type it cannot read is rejected at load with a structured error naming the
// role and type — you must read a type to modify or remove it. CREATE is EXEMPT:
// a role may create a type it cannot read (it reads back only what it authored
// via a role-conferring relation), so create-without-read loads fine.
func TestLoadPolicy_WriteWithoutRead_Rejected(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr bool
		// substrings the error must carry so the operator can find
		// the offending role without reading source code.
		wantInErr []string
	}{
		{
			name:      "update without read rejected",
			yaml:      "roles:\n  triager:\n    update: [ticket]\n",
			wantErr:   true,
			wantInErr: []string{"triager", "ticket", "read"},
		},
		{
			name:      "delete without read rejected",
			yaml:      "roles:\n  triager:\n    delete: [ticket]\n",
			wantErr:   true,
			wantInErr: []string{"triager", "ticket", "read"},
		},
		{
			// CREATE is exempt — this is the TKT-4LQMWP submitter case.
			name:    "create without read OK (create is exempt)",
			yaml:    "roles:\n  submitter:\n    create: [ticket]\n",
			wantErr: false,
		},
		{
			name:      "wildcard update without wildcard read rejected",
			yaml:      "roles:\n  admin:\n    update: [\"*\"]\n    read: [ticket]\n",
			wantErr:   true,
			wantInErr: []string{"admin", "read"},
		},
		{
			name:    "update covered by exact read ok",
			yaml:    "roles:\n  editor:\n    update: [ticket]\n    read: [ticket]\n",
			wantErr: false,
		},
		{
			name:    "update covered by wildcard read ok",
			yaml:    "roles:\n  editor:\n    update: [ticket]\n    read: [\"*\"]\n",
			wantErr: false,
		},
		{
			name:    "full CUD covered by wildcard read ok",
			yaml:    "roles:\n  admin:\n    create: [\"*\"]\n    update: [\"*\"]\n    delete: [\"*\"]\n    read: [\"*\"]\n",
			wantErr: false,
		},
		{
			name:    "read-only role ok",
			yaml:    "roles:\n  viewer:\n    read: [ticket]\n",
			wantErr: false,
		},
		{
			name:    "empty role ok",
			yaml:    "roles:\n  nobody: {}\n",
			wantErr: false,
		},
		{
			// Affordance grants restrict surfaces within a write the
			// verb grant authorized; they never authorize by
			// themselves, so they are intentionally outside the
			// update/delete⊆read invariant (see Validate godoc).
			name: "affordance-only role without read ok",
			yaml: "roles:\n  shaper:\n    fields:\n      ticket:\n        - field: status\n" +
				"    relations:\n      ticket:\n        - relation: implements\n          create: true\n",
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := acl.LoadPolicy(writeTempPolicy(t, tc.yaml))
			if tc.wantErr {
				if err == nil {
					t.Fatal("LoadPolicy: expected write-without-read rejection; got nil")
				}
				for _, want := range tc.wantInErr {
					if !contains(err.Error(), want) {
						t.Errorf("error %q missing substring %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadPolicy: unexpected error: %v", err)
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
