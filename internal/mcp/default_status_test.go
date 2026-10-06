package mcp

import (
	"context"
	"log/slog"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestHandleCreateEntity_StatusDefaultComesOnlyFromConfig pins that MCP
// create_entity sets a missing status only when the schema declares a
// default (BUG-ZD4PIN).
func TestHandleCreateEntity_StatusDefaultComesOnlyFromConfig(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(`
version: "1.0"
types:
  declared:
    values: [open, closed]
    default: closed
  undeclared:
    values: [open, closed]
entities:
  note:
    label: Note
    id_prefix: "N-"
    id_type: sequential
    properties:
      title: {type: string}
  type_default:
    label: TypeDefault
    id_prefix: "TD-"
    id_type: sequential
    properties:
      title: {type: string}
      status: {type: declared}
  no_default:
    label: NoDefault
    id_prefix: "ND-"
    id_type: sequential
    properties:
      title: {type: string}
      status: {type: undeclared}
relations: {}
`))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	s := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(s, newTestDeps(t, meta, st))

	for _, tc := range []struct {
		typ, id, want string // want "" means absent
	}{
		{"note", "N-001", ""},
		{"type_default", "TD-001", "closed"},
		{"no_default", "ND-001", ""},
	} {
		res, createErr := s.handleCreateEntity(context.Background(), makeToolRequest(map[string]any{
			"type": tc.typ, "properties": map[string]any{"title": "x"},
		}))
		if createErr != nil || res.IsError {
			t.Fatalf("%s: create failed: %v %v", tc.typ, createErr, res)
		}
		e, getErr := st.GetEntity(context.Background(), entity.Ref{ID: tc.id})
		if getErr != nil {
			t.Fatalf("%s: %v", tc.id, getErr)
		}
		got, present := e.Properties["status"]
		switch {
		case tc.want == "" && present:
			t.Errorf("%s: status = %v, want absent", tc.typ, got)
		case tc.want != "" && got != tc.want:
			t.Errorf("%s: status = %v, want %q", tc.typ, got, tc.want)
		}
	}

	// rela.create_entity through lua_eval uses the same manager.
	res, err := group(s, selLua).handleLuaEval(context.Background(),
		makeToolRequest(map[string]any{"code": `rela.create_entity("note", {title = "from lua"})`}))
	if err != nil || res.IsError {
		t.Fatalf("lua create failed: %v %s", err, getResultText(t, res))
	}
	e, err := st.GetEntity(context.Background(), entity.Ref{ID: "N-002"})
	if err != nil {
		t.Fatalf("N-002: %v", err)
	}
	if got, ok := e.Properties["status"]; ok {
		t.Errorf("lua-created note: status = %v, want absent", got)
	}
}
