package affordances_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// faceBlockMeta declares a document with a concept and an adopted face
// (vastgesteld), and a faceless note.
func faceBlockMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"document": {
				Properties: map[string]metamodel.PropertyDef{
					"titel":       {Type: metamodel.PropertyTypeString},
					"tekst":       {Type: metamodel.PropertyTypeString},
					"ondertekend": {Type: metamodel.PropertyTypeString},
					"status":      {Type: metamodel.PropertyTypeEnum, Values: []string{"open", "gesloten"}},
				},
				Faces: map[string]metamodel.FaceDef{"concept": {}, "vastgesteld": {}},
			},
			"note":   {Properties: map[string]metamodel.PropertyDef{"text": {Type: metamodel.PropertyTypeString}}},
			"person": {Properties: map[string]metamodel.PropertyDef{}},
		},
		Relations: map[string]metamodel.RelationDef{
			"cites": {From: []string{"document"}, To: []string{"document"}},
		},
	}
}

// faceBlockPolicy: `signer` edits concepts freely and writes only the
// signature on an adopted document. `mixed` holds a type block and a face
// block, which add up on the face. `chooser` restricts status options on
// the adopted face only.
const faceBlockPolicy = `
roles:
  signer:
    read: [document]
    create: ["document@concept", "document@vastgesteld"]
    update: ["document@concept", "document@vastgesteld"]
    fields:
      document@vastgesteld:
        - field: ondertekend
  mixed:
    read: [document]
    update: ["document@concept", "document@vastgesteld"]
    fields:
      document:
        - field: titel
      document@vastgesteld:
        - field: ondertekend
  chooser:
    read: [document]
    update: ["document@concept", "document@vastgesteld"]
    options:
      document@vastgesteld:
        - field: status
          option: gesloten
assignments:
  sam: signer
  max: mixed
  cho: chooser
`

func newFaceBlockResolver(t *testing.T, policyYAML string) (*affordances.PolicyResolver, *acl.Declarative, error) {
	t.Helper()
	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	ms := memstore.New()
	for _, user := range []string{"sam", "max", "cho", "gus"} {
		mustCreateEntity(t, ms, user, "person")
	}
	decl, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(faceBlockMeta(), storeRelationLookup{ms}, decl)
	return resolver, decl, err
}

func faceBlockDocument(face entity.Face) *entity.Entity {
	e := entity.New("DOC-1", "document")
	e.Face = face
	return e
}

// TestFieldVerdicts_FaceBlocks pins `type@face` keys in `fields:` and
// `options:` (TKT-RSVJKS): a face block counts on rows of its face only, a
// role is block-less on the other faces, and a type block and a face block of
// one role add up.
func TestFieldVerdicts_FaceBlocks(t *testing.T) {
	t.Parallel()
	resolver, decl, err := newFaceBlockResolver(t, faceBlockPolicy)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	update := resolver.FieldVerdicts
	create := resolver.CreateFieldVerdicts

	tests := []struct {
		name     string
		user     string
		verdicts func(context.Context, *entity.Entity) affordances.FieldVerdicts
		face     entity.Face
		readOnly []string
		options  []string
	}{
		{
			name: "face block restricts its face", user: "sam", verdicts: update, face: "vastgesteld",
			readOnly: []string{"status", "tekst", "titel"},
		},
		{
			name: "face block leaves another face open", user: "sam", verdicts: update, face: "concept",
		},
		{
			name: "face block restricts its face on create", user: "sam", verdicts: create, face: "vastgesteld",
			readOnly: []string{"status", "tekst", "titel"},
		},
		{
			name: "type block alone on another face", user: "max", verdicts: update, face: "concept",
			readOnly: []string{"ondertekend", "status", "tekst"},
		},
		{
			name: "type and face blocks add up on the face", user: "max", verdicts: update, face: "vastgesteld",
			readOnly: []string{"status", "tekst"},
		},
		{
			name: "face options block restricts its face", user: "cho", verdicts: update, face: "vastgesteld",
			options: []string{"status=open"},
		},
		{
			name: "face options block leaves another face open", user: "cho", verdicts: update, face: "concept",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fv := tc.verdicts(userCtx(t, decl, tc.user), faceBlockDocument(tc.face))
			if got := denied(fv.Writable); !slices.Equal(got, tc.readOnly) {
				t.Errorf("read-only = %v, want %v", got, tc.readOnly)
			}
			if got := deniedOptionPaths(fv.Options); !slices.Equal(got, tc.options) {
				t.Errorf("denied options = %v, want %v", got, tc.options)
			}
			if got := denied(fv.Visible); len(got) != 0 {
				t.Errorf("hidden = %v, want none", got)
			}
		})
	}
}

// TestFieldVerdicts_FaceBlocksAcrossRoles pins the union across roles: a
// role with only a face block gives nothing on a face it may not update, and
// on its face its block adds to another role's type block. A `when:` in a
// face block is judged on the row as usual.
func TestFieldVerdicts_FaceBlocksAcrossRoles(t *testing.T) {
	t.Parallel()
	const policy = `
roles:
  everyone:
    read: [document]
    update: ["document@concept", "document@vastgesteld"]
    fields:
      document:
        - field: titel
  signer:
    read: [document]
    update: ["document@vastgesteld"]
    fields:
      document@vastgesteld:
        - field: ondertekend
  gated:
    read: [document]
    update: ["document@vastgesteld"]
    fields:
      document@vastgesteld:
        - field: ondertekend
          when: "entity.status == 'open'"
assignments:
  sam: signer
  gus: gated
`
	resolver, decl, err := newFaceBlockResolver(t, policy)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	tests := []struct {
		name     string
		user     string
		face     entity.Face
		status   string
		readOnly []string
	}{
		{name: "no update on the face gives nothing there", user: "sam", face: "concept",
			readOnly: []string{"ondertekend", "status", "tekst"}},
		{name: "face block adds to another role's type block", user: "sam", face: "vastgesteld",
			readOnly: []string{"status", "tekst"}},
		{name: "when: holds on the row", user: "gus", face: "vastgesteld", status: "open",
			readOnly: []string{"status", "tekst"}},
		{name: "when: fails on the row", user: "gus", face: "vastgesteld", status: "gesloten",
			readOnly: []string{"ondertekend", "status", "tekst"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			row := faceBlockDocument(tc.face)
			if tc.status != "" {
				row.Properties = map[string]any{"status": tc.status}
			}
			fv := resolver.FieldVerdicts(userCtx(t, decl, tc.user), row)
			if got := denied(fv.Writable); !slices.Equal(got, tc.readOnly) {
				t.Errorf("read-only = %v, want %v", got, tc.readOnly)
			}
		})
	}
}

// TestNew_FaceBlockKeyErrors pins the load errors for block keys that name a
// face: only `fields:` and `options:` accept one, and it must be a face the
// type declares.
func TestNew_FaceBlockKeyErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		policy string
		want   string
	}{
		{
			name:   "undeclared face",
			policy: "fields:\n      document@draft:\n        - field: titel",
			want:   `type "document" declares no face "draft" (declared: concept, vastgesteld)`,
		},
		{
			name:   "faceless type",
			policy: "fields:\n      note@draft:\n        - field: text",
			want:   `type "note" declares no faces; use the key "note"`,
		},
		{
			name:   "unknown type",
			policy: "options:\n      nope@draft: []",
			want:   `roles.r.options.nope@draft: unknown entity type "nope"`,
		},
		{
			name:   "empty face",
			policy: "fields:\n      document@: []",
			want:   "roles.r.fields.document@: empty face",
		},
		{
			name:   "two separators",
			policy: "fields:\n      document@concept@x: []",
			want:   "a face cannot have a face",
		},
		{
			name:   "visible refuses a face key",
			policy: "visible:\n      document@concept:\n        - field: titel",
			want:   "a visible: block is keyed by entity type only",
		},
		{
			name:   "relations refuse a face key",
			policy: "relations:\n      document@concept:\n        - relation: cites",
			want:   "a relations: block is keyed by entity type only",
		},
		{
			name:   "unknown field names the key",
			policy: "fields:\n      document@concept:\n        - field: nope",
			want:   `roles.r.fields.document@concept[0]: unknown field "nope" on type "document"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			policy := "roles:\n  r:\n    read: [document]\n    " + tc.policy + "\n"
			_, _, err := newFaceBlockResolver(t, policy)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("New error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
