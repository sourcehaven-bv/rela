package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"iter"
	"log/slog"
	"strings"
	"testing"

	mcpgo "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// failingCountReader wraps the server's GraphReader and fails every
// CountRelations call, simulating a backend outage during the scan.
//
// GraphReader must be non-nil — every method this does not override
// delegates to it, so a zero value panics at the first such call rather
// than at construction. Build one with newFailingCountReader.
type failingCountReader struct {
	GraphReader
	err error
}

func newFailingCountReader(t *testing.T, base GraphReader, err error) failingCountReader {
	t.Helper()
	if base == nil {
		t.Fatal("failingCountReader needs a non-nil base reader")
	}
	return failingCountReader{GraphReader: base, err: err}
}

func (f failingCountReader) CountRelations(context.Context, store.RelationQuery) (int, error) {
	return 0, f.err
}

// TestHandleAnalyzeCardinality_CountErrorFailsTheToolCall pins the reason
// TKT-CICJSN deleted the MCP-local copy: that copy did `count, _ :=
// CountRelations(...)`, so an outage counted as 0 and a min bound turned it
// into a fabricated "missing relations" violation. The shared checker fails
// the tool call instead, and must never report a violation computed from a
// failed count.
func TestHandleAnalyzeCardinality_CountErrorFailsTheToolCall(t *testing.T) {
	t.Parallel()

	meta, st := makeTestFixture(t)
	// DEC-001 genuinely HAS its one `addresses` edge, so only the outage
	// could manufacture a violation here.
	one := 1
	meta.Relations["addresses"] = metamodel.RelationDef{
		Label: "addresses", From: []string{"decision"}, To: []string{"requirement"},
		MinOutgoing: &one,
	}

	deps := newTestDeps(t, meta, st)
	countErr := errors.New("backend down")
	deps.Store = newFailingCountReader(t, deps.Store, countErr)

	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, deps)

	result, err := srv.handleAnalyzeCardinality(context.Background(), &mcpgo.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if !isErrorResult(result) {
		t.Fatalf("want an error result, got: %s", getResultText(t, result))
	}
	text := getResultText(t, result)
	if !strings.Contains(text, countErr.Error()) {
		t.Errorf("error result does not carry the store error: %s", text)
	}
	if strings.Contains(text, "must have at least") {
		t.Errorf("reported a violation computed from a failed count: %s", text)
	}
}

// TestHandleAnalyzeCardinality_TruncatedScanStillAnswers covers the second
// defect TKT-CICJSN names: the deleted copy did `if err != nil { break }` on
// the ListEntities iterator.
//
// The RESULT is deliberately unchanged by the fix — an under-count can only
// miss findings, never invent them, so the tool still answers rather than
// failing (unlike a failed count, which does invent them, and which the test
// above pins). What the fix added is a log record, and that is asserted in
// TestCheckCardinality_TruncatedScanIsLogged over in internal/schema, where
// no test runs in parallel and swapping the default logger is safe. This test
// stays parallel and covers the MCP-level contract: answer, don't fabricate.
func TestHandleAnalyzeCardinality_TruncatedScanStillAnswers(t *testing.T) {
	t.Parallel()

	meta, st := makeTestFixture(t)
	one := 1
	meta.Relations["addresses"] = metamodel.RelationDef{
		Label: "addresses", From: []string{"decision"}, To: []string{"requirement"},
		MinOutgoing: &one,
	}

	deps := newTestDeps(t, meta, st)
	deps.Store = newFailingListReader(t, deps.Store, errors.New("scan interrupted"))

	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, deps)

	result, err := srv.handleAnalyzeCardinality(context.Background(), &mcpgo.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if isErrorResult(result) {
		t.Fatalf("a truncated scan must not fail the call: %s", getResultText(t, result))
	}
	if text := getResultText(t, result); strings.Contains(text, "violation") {
		t.Errorf("invented violations for rows never scanned: %s", text)
	}
}

// failingListReader yields the iterator error on the first row, so nothing
// is ever scanned.
//
// GraphReader must be non-nil, for the same reason as failingCountReader.
// Build one with newFailingListReader.
type failingListReader struct {
	GraphReader
	err error
}

func newFailingListReader(t *testing.T, base GraphReader, err error) failingListReader {
	t.Helper()
	if base == nil {
		t.Fatal("failingListReader needs a non-nil base reader")
	}
	return failingListReader{GraphReader: base, err: err}
}

func (f failingListReader) ListEntities(
	context.Context, store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		yield(nil, f.err)
	}
}

// TestHandleAnalyzeCardinality_IncomingUsesInverseLabel pins the wording the
// MCP surface adopted from the shared checker: an incoming bound is reported
// against the relation's INVERSE id, which is the name the subject entity's
// own operator would use, rather than the forward name with an "incoming "
// prefix. Both surfaces render through CardinalityViolation.Message(), so the
// sentence is the same one `rela analyze cardinality` prints; the CLI puts the
// entity id in front of it, while MCP carries the id in its own JSON field.
func TestHandleAnalyzeCardinality_IncomingUsesInverseLabel(t *testing.T) {
	t.Parallel()

	meta, st := makeTestFixture(t)
	one := 1
	meta.Relations["addresses"] = metamodel.RelationDef{
		Label: "addresses", From: []string{"decision"}, To: []string{"requirement"},
		MinIncoming: &one,
		Inverse:     &metamodel.InverseDef{ID: "addressed-by"},
	}

	deps := newTestDeps(t, meta, st)
	srv := &Server{logger: slog.New(slog.DiscardHandler)}
	setDeps(srv, deps)

	result, err := srv.handleAnalyzeCardinality(context.Background(), &mcpgo.CallToolRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := getResultText(t, result)

	var violations []struct {
		EntityID string `json:"entity_id"`
		Relation string `json:"relation"`
		Message  string `json:"message"`
	}
	start := strings.Index(text, "[")
	if start < 0 {
		t.Fatalf("no JSON payload in result: %s", text)
	}
	if err := json.Unmarshal([]byte(text[start:]), &violations); err != nil {
		t.Fatalf("parse payload %q: %v", text, err)
	}
	// REQ-002 and REQ-003 have no incoming `addresses` edge; REQ-001 does.
	if len(violations) != 2 {
		t.Fatalf("want 2 violations, got %d: %s", len(violations), text)
	}
	for _, v := range violations {
		if v.Relation != "addressed-by" {
			t.Errorf("%s: want inverse label %q, got %q", v.EntityID, "addressed-by", v.Relation)
		}
		if !strings.Contains(v.Message, "'addressed-by'") {
			t.Errorf("%s: message does not use the inverse label: %s", v.EntityID, v.Message)
		}
	}
}
