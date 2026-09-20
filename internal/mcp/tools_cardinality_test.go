package mcp

import (
	"bytes"
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
type failingCountReader struct {
	GraphReader
	err error
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
	deps.Store = failingCountReader{GraphReader: deps.Store, err: countErr}

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

// TestHandleAnalyzeCardinality_TruncatedScanIsLogged covers the second
// defect TKT-CICJSN names: the deleted copy did `if err != nil { break }` on
// the ListEntities iterator, so a truncated scan was presented as a complete
// one with nothing anywhere recording that rows were missed.
//
// The RESULT is deliberately the same either way — an under-count can only
// miss findings, never invent them, so the tool still answers rather than
// failing (unlike a failed count, which does invent them). What changed is
// that the shared checker leaves a trace, which is the whole difference
// between a quiet wrong answer and a diagnosable one. The assertion is on
// the log for exactly that reason.
func TestHandleAnalyzeCardinality_TruncatedScanIsLogged(t *testing.T) {
	meta, st := makeTestFixture(t)
	one := 1
	meta.Relations["addresses"] = metamodel.RelationDef{
		Label: "addresses", From: []string{"decision"}, To: []string{"requirement"},
		MinOutgoing: &one,
	}

	deps := newTestDeps(t, meta, st)
	scanErr := errors.New("scan interrupted")
	deps.Store = failingListReader{GraphReader: deps.Store, err: scanErr}

	// collectCardinalitySubjects warns on the package logger, so capture the
	// default for the duration of the call. Hence no t.Parallel() here.
	var logged bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(restore) })

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
	if !strings.Contains(logged.String(), scanErr.Error()) {
		t.Errorf("truncated scan left no trace in the log; got: %s", logged.String())
	}
}

// failingListReader yields the iterator error on the first row, so nothing
// is ever scanned.
type failingListReader struct {
	GraphReader
	err error
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
// prefix. Same string `rela analyze cardinality` prints.
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
