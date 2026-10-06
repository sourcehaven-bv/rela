package entitymanager_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// defaultStatusMetaYAML covers every way a schema can (or cannot) declare
// the status a new entity starts in. BUG-ZD4PIN: rela must never invent one.
const defaultStatusMetaYAML = `
version: "1.0"
types:
  declared:
    values: [open, closed]
    default: closed
  undeclared:
    values: [open, closed]
  machine:
    values: [active, new, done]
    initial: new
    transitions:
      - {from: new, to: active}
      - {from: active, to: done}
entities:
  note:
    label: Note
    id_prefix: "N-"
    id_type: sequential
    properties:
      title: {type: string}
  prop_default:
    label: PropDefault
    id_prefix: "PD-"
    id_type: sequential
    properties:
      status: {type: undeclared, default: open}
  type_default:
    label: TypeDefault
    id_prefix: "TD-"
    id_type: sequential
    properties:
      status: {type: declared}
  machine_entry:
    label: MachineEntry
    id_prefix: "ME-"
    id_type: sequential
    properties:
      status: {type: machine}
  no_default:
    label: NoDefault
    id_prefix: "ND-"
    id_type: sequential
    properties:
      status: {type: undeclared}
  inline_enum:
    label: InlineEnum
    id_prefix: "IE-"
    id_type: sequential
    properties:
      status: {type: enum, values: [x, y]}
  required_no_default:
    label: RequiredNoDefault
    id_prefix: "RN-"
    id_type: sequential
    properties:
      status: {type: undeclared, required: true}
relations: {}
`

func newDefaultStatusManager(t *testing.T) *entitymanager.Manager {
	t.Helper()
	meta, err := metamodel.Parse([]byte(defaultStatusMetaYAML))
	if err != nil {
		t.Fatal(err)
	}
	set, err := statemachine.Compile(meta)
	if err != nil {
		t.Fatal(err)
	}
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: memstore.New(), Meta: meta, Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: set,
		FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatal(err)
	}
	return mgr
}

// TestCreate_StatusDefaultComesOnlyFromConfig pins that a create without a
// status sets one only when the schema declares it (BUG-ZD4PIN).
func TestCreate_StatusDefaultComesOnlyFromConfig(t *testing.T) {
	tests := []struct {
		entityType string
		want       string // "" means the property must be absent
	}{
		{"note", ""},
		{"prop_default", "open"},
		{"type_default", "closed"},
		{"machine_entry", "new"},
		{"no_default", ""},
		{"inline_enum", ""},
		{"required_no_default", ""},
	}
	for _, tc := range tests {
		t.Run(tc.entityType, func(t *testing.T) {
			mgr := newDefaultStatusManager(t)
			res, err := mgr.CreateEntity(context.Background(), entity.New("", tc.entityType), entity.CreateOptions{})
			if err != nil {
				t.Fatalf("CreateEntity: %v", err)
			}
			got, present := res.Entity.Properties["status"]
			if tc.want == "" {
				if present {
					t.Fatalf("status = %v, want absent", got)
				}
				return
			}
			if got != tc.want {
				t.Fatalf("status = %v, want %q", got, tc.want)
			}
		})
	}
}

// TestCreate_RequiredStatusWithoutDefaultWarns pins that a required status
// with no declared default is reported by validation, not filled in.
func TestCreate_RequiredStatusWithoutDefaultWarns(t *testing.T) {
	mgr := newDefaultStatusManager(t)
	res, err := mgr.CreateEntity(context.Background(), entity.New("", "required_no_default"), entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	for _, w := range res.Warnings {
		if strings.Contains(w.Path, "status") || strings.Contains(w.Detail, "status") {
			return
		}
	}
	t.Fatalf("want a warning naming status, got %+v", res.Warnings)
}

// TestCreate_ExplicitStatusIsKept pins that a caller's value always wins.
func TestCreate_ExplicitStatusIsKept(t *testing.T) {
	mgr := newDefaultStatusManager(t)
	e := entity.New("", "type_default")
	e.Properties["status"] = "open"
	res, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
	if err != nil {
		t.Fatalf("CreateEntity: %v", err)
	}
	if got := res.Entity.GetString("status"); got != "open" {
		t.Fatalf("status = %q, want open", got)
	}
}

// TestCreate_EmptyStatus pins that an explicit "" is filled from a declared
// default and dropped when there is none, never stored as "".
func TestCreate_EmptyStatus(t *testing.T) {
	for _, tc := range []struct{ entityType, want string }{
		{"type_default", "closed"},
		{"no_default", ""},
	} {
		t.Run(tc.entityType, func(t *testing.T) {
			mgr := newDefaultStatusManager(t)
			e := entity.New("", tc.entityType)
			e.Properties["status"] = ""
			res, err := mgr.CreateEntity(context.Background(), e, entity.CreateOptions{})
			if err != nil {
				t.Fatalf("CreateEntity: %v", err)
			}
			got, present := res.Entity.Properties["status"]
			if tc.want == "" && present {
				t.Fatalf("status = %q, want absent", got)
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("status = %v, want %q", got, tc.want)
			}
		})
	}
}
