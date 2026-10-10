package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// failingCounts is a GraphReader whose counts fail, as a gated count does on
// a gate fault.
type failingCounts struct{ GraphReader }

var errCountFault = errors.New("count fault")

func (failingCounts) CountEntities(context.Context, store.EntityQuery) (int, error) {
	return 0, errCountFault
}

func (failingCounts) CountRelations(context.Context, store.RelationQuery) (int, error) {
	return 0, errCountFault
}

// A failed count must not read as zero rows (TKT-QZTROQ): the schema tool
// leaves the count out, and the summary prompt fails.
func TestSchemaCounts_FailureIsNotZero(t *testing.T) {
	t.Parallel()
	s := makeTestServer(t)
	ctx := context.Background()

	h := group(s, selSchemaRes)
	h.store = failingCounts{h.store}
	result, err := h.handleSchema(ctx, &mcpgo.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleSchema: %v", err)
	}
	var parsed struct {
		EntityTypes   []map[string]any `json:"entity_types"`
		RelationTypes []map[string]any `json:"relation_types"`
	}
	if err := json.Unmarshal([]byte(getResultText(t, result)), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.EntityTypes) == 0 || len(parsed.RelationTypes) == 0 {
		t.Fatalf("overview lost its types: %+v", parsed)
	}
	for _, row := range append(parsed.EntityTypes, parsed.RelationTypes...) {
		if c, ok := row["count"]; ok {
			t.Errorf("%v: count = %v, want it left out", row["name"], c)
		}
	}

	p := group(s, selPrompts)
	p.store = failingCounts{p.store}
	if _, err := p.handleSummarizeProjectPrompt(ctx, &mcpgo.GetPromptRequest{}); !errors.Is(err, errCountFault) {
		t.Errorf("summary prompt err = %v, want the count fault", err)
	}
}
