package mcp

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TKT-SM20FG finding 2: lua_eval runs client-supplied code, so its runtime
// is not granted external-ref writes. A create or update that sets a ref is
// refused, and the stored ref is unchanged.
func TestLuaEval_ExternalRefWriteRefused(t *testing.T) {
	t.Parallel()
	meta, err := metamodel.Parse([]byte(`
version: "1.0"
entities:
  ticket:
    label: Ticket
    id_type: manual
    properties:
      title: {type: string}
      jira: {type: external_ref, system: jira}
relations: {}
`))
	if err != nil {
		t.Fatal(err)
	}
	st := memstore.New()
	ctx := context.Background()
	seed := entity.New("T-1", "ticket")
	seed.Properties["jira"] = map[string]any{"id": "J-1"}
	if cerr := st.CreateEntity(ctx, seed); cerr != nil {
		t.Fatal(cerr)
	}
	s := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(s, newTestDeps(t, meta, st))

	for _, code := range []string{
		`rela.create_entity("ticket", {title = "x", jira = {id = "J-2"}}, "T-2")`,
		`rela.update_entity("T-1", {jira = {id = "J-9"}})`,
	} {
		res, evalErr := group(s, selLua).handleLuaEval(ctx, makeToolRequest(map[string]any{"code": code}))
		if evalErr != nil {
			t.Fatalf("%s: %v", code, evalErr)
		}
		if text := getResultText(t, res); !res.IsError || !strings.Contains(text, "external ref") {
			t.Errorf("%s: want the external-ref refusal, got %s", code, text)
		}
	}
	got, err := st.GetEntity(ctx, entity.Ref{ID: "T-1"})
	if err != nil {
		t.Fatal(err)
	}
	if metamodel.FormatExternalRef(got.Properties["jira"]) != "J-1" {
		t.Errorf("stored ref = %v, want J-1", got.Properties["jira"])
	}
	if _, err := st.GetEntity(ctx, entity.Ref{ID: "T-2"}); err == nil {
		t.Error("T-2 was created")
	}
}
