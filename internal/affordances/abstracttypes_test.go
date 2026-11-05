package affordances_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/acl/acltest"
	"github.com/Sourcehaven-BV/rela/internal/aclstore"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// These tests run an acl.yaml that names an abstract type through the real
// read path: the loaded schema, ValidateAgainstMetamodel (which expands the
// abstract names), NewDeclarative, and FieldVerdicts or ReadQuery. The acl
// package's own tests pin the expanded policy; these pin what a reader then
// sees, so a wrong expansion shows up as a visible field.

const variantSchemaRoot = `
version: "1.0"
includes: [isms.yaml, projects.yaml]
entities:
  person:
    label: Person
    id_prefix: "P-"
    properties:
      name: { type: string }
  task:
    abstract: {}
    label: Task
    properties:
      title: { type: string }
      salary: { type: string }
      due: { type: string }
`

const variantSchemaISMS = `
namespace: isms
entities:
  task:
    extends: task
    id_prefix: "ISMS-"
`

const variantSchemaProjects = `
namespace: projects
entities:
  task:
    extends: task
    id_prefix: "PRJ-"
`

type variantWorld struct {
	resolver *affordances.PolicyResolver
	decl     *acl.Declarative
	ismsTask *entity.Entity
	projTask *entity.Entity
}

func newVariantWorld(t *testing.T, policyYAML string) variantWorld {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"schema.yaml": variantSchemaRoot, "isms.yaml": variantSchemaISMS, "projects.yaml": variantSchemaProjects,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	meta, _, err := metamodel.Load(filepath.Join(dir, "schema.yaml"), storage.NewOsFS())
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	policy, err := acl.LoadPolicyBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	if verr := policy.ValidateAgainstMetamodel(variantMetaView{meta}); verr != nil {
		t.Fatalf("ValidateAgainstMetamodel: %v", verr)
	}

	ms := memstore.New()
	mustCreateEntity(t, ms, "alice", "person")
	isms := entity.New("ISMS-1", "isms.task")
	proj := entity.New("PRJ-1", "projects.task")
	for _, e := range []*entity.Entity{isms, proj} {
		e.SetString("title", "T")
		e.SetString("salary", "100000")
		e.SetString("due", "2026-12-01")
		mustCreate(t, ms, e)
	}
	decl, err := acl.NewDeclarative(policy, aclstore.MustNewGraph(ms), ms)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	resolver, err := affordances.New(meta, storeRelationLookup{ms}, decl)
	if err != nil {
		t.Fatalf("affordances.New: %v", err)
	}
	return variantWorld{resolver: resolver, decl: decl, ismsTask: isms, projTask: proj}
}

func (w variantWorld) ctxFor(t *testing.T, principalType string) context.Context {
	t.Helper()
	ctx, _ := w.requestFor(t, principalType)
	return ctx
}

func (w variantWorld) requestFor(t *testing.T, principalType string) (context.Context, *acl.Request) {
	t.Helper()
	p := principal.VerifiedFrom("alice", principal.ToolDataEntry, principal.Claims{PrincipalType: principalType})
	req, err := w.decl.ForIdentity(acltest.IdentityFor(t.Context(), t, w.decl, p))
	if err != nil {
		t.Fatalf("ForIdentity: %v", err)
	}
	return acl.WithRequest(principal.With(context.Background(), p), req), req
}

// variantMetaView adapts the loaded schema to acl.MetamodelView the way the
// wiring's adapter does.
type variantMetaView struct{ m *metamodel.Metamodel }

func (v variantMetaView) HasEntityType(t string) bool { return v.m.HasEntityType(t) }
func (v variantMetaView) DefaultWorld() string        { return metamodel.EffectiveDefaultWorld(v.m) }
func (v variantMetaView) Variants(t string) ([]string, bool) {
	return metamodel.VariantsOf(v.m, t)
}

func (v variantMetaView) FaceNames(t string) (canonical string, faces []string) {
	return v.m.ResolveAlias(t), metamodel.FaceOrderOf(v.m, t)
}

func (v variantMetaView) RelationInfo(string) acl.RelationInfo { return acl.RelationInfo{} }

func (v variantMetaView) PropertyInfo(entityType, property string) acl.PropertyInfo {
	pd, ok := v.m.Entities[entityType].Properties[property]
	if !ok {
		return acl.PropertyInfo{}
	}
	return acl.PropertyInfo{Exists: true, Unique: pd.Unique, List: pd.List}
}

type fieldCase struct {
	row    string // "isms" or "projects"
	field  string
	hidden bool
}

func (w variantWorld) checkFields(ctx context.Context, t *testing.T, cases []fieldCase) {
	t.Helper()
	for _, tc := range cases {
		row := w.ismsTask
		if tc.row == "projects" {
			row = w.projTask
		}
		if got := hidden(t, w.resolver.FieldVerdicts(ctx, row), tc.field); got != tc.hidden {
			t.Errorf("%s %s: hidden = %v, want %v", row.Type, tc.field, got, tc.hidden)
		}
	}
}

// TestAbstractACL_RoleVisibleVariantWins pins that a role's `visible:` entry
// for a variant replaces the abstract entry rather than adding to it. The
// variant entry here was written to keep salary from this role.
func TestAbstractACL_RoleVisibleVariantWins(t *testing.T) {
	t.Parallel()
	w := newVariantWorld(t, `
roles:
  viewer:
    read: [task, person]
    visible:
      task:
        - field: title
        - field: salary
      isms.task:
        - field: title
assignments:
  alice: viewer
`)
	w.checkFields(w.ctxFor(t, "user"), t, []fieldCase{
		{"isms", "title", false},
		{"isms", "salary", true},
		{"isms", "due", true},
		{"projects", "title", false},
		{"projects", "salary", false},
		{"projects", "due", true},
	})
}

// TestAbstractACL_CeilingRedactSurvivesVariantVisible pins the fail-open case
// for client ceilings: a `redact:` on the abstract type still hides the field
// on a variant whose own `visible:` names it.
func TestAbstractACL_CeilingRedactSurvivesVariantVisible(t *testing.T) {
	t.Parallel()
	w := newVariantWorld(t, hrReadsTasks+`
client_baselines:
  apps:
    applies_to: [app]
    redact:
      task: [salary]
    visible:
      isms.task: [title, salary, due]
`)
	w.checkFields(w.ctxFor(t, "app"), t, []fieldCase{
		{"isms", "salary", true},
		{"isms", "due", false},
		{"projects", "salary", true},
		{"projects", "due", false},
	})
	// The same user without the client ceiling sees every field.
	w.checkFields(w.ctxFor(t, "user"), t, []fieldCase{
		{"isms", "salary", false},
		{"projects", "salary", false},
	})
}

// TestAbstractACL_CeilingVisibleVariantWins pins that a variant's ceiling
// `visible:` replaces the abstract one, and that the abstract allowlist is
// closed on the other variants.
func TestAbstractACL_CeilingVisibleVariantWins(t *testing.T) {
	t.Parallel()
	w := newVariantWorld(t, hrReadsTasks+`
client_baselines:
  apps:
    applies_to: [app]
    visible:
      task: [title]
      isms.task: [title, due]
`)
	w.checkFields(w.ctxFor(t, "app"), t, []fieldCase{
		{"isms", "due", false},
		{"isms", "salary", true},
		{"projects", "title", false},
		{"projects", "due", true},
		{"projects", "salary", true},
	})
}

const hrReadsTasks = `
roles:
  hr:
    read: [task, person]
assignments:
  alice: hr
`

// TestAbstractACL_DenyReadCoversVariants pins that a ceiling's deny on the
// abstract type denies every variant, and that a read grant on the abstract
// type reaches no type outside it.
func TestAbstractACL_DenyReadCoversVariants(t *testing.T) {
	t.Parallel()
	w := newVariantWorld(t, `
roles:
  reader:
    read: [task]
assignments:
  alice: reader
client_baselines:
  apps:
    applies_to: [app]
    deny_read: [task]
`)
	user, req := w.requestFor(t, "user")
	for _, typ := range []string{"isms.task", "projects.task"} {
		if got := req.ReadQuery(user, typ); got.DenyAll {
			t.Errorf("user ReadQuery(%s) denies all, want the abstract read grant to reach it", typ)
		}
	}
	if got := req.ReadQuery(user, "person"); !got.DenyAll {
		t.Errorf("user ReadQuery(person) = %+v, want denied: person is not a variant of task", got)
	}

	app, appReq := w.requestFor(t, "app")
	for _, typ := range []string{"isms.task", "projects.task"} {
		if got := appReq.ReadQuery(app, typ); !got.DenyAll {
			t.Errorf("app ReadQuery(%s) = %+v, want denied by deny_read on the abstract type", typ, got)
		}
	}
}
