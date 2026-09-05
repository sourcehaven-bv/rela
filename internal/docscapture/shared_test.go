package docscapture

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

func seedTicket(id, title string) docs.SeedOp {
	return docs.SeedOp{
		Kind: "create", Type: "ticket", ID: id,
		Properties: map[string]any{
			"title": title, "status": "open", "priority": "low", "reporter": "a@b.c",
		},
	}
}

// The whole point of the change: a write issued through api{} is visible to a
// LATER island, because both reach one project.
//
// Before this, api{} and screenshot{} each stood up their own temp project, so
// a POST on one side landed in a store the other never read — which is why the
// worlds manual could assert that publishing is a copy and then only DESCRIBE
// the result in prose, never photograph it.
func TestSharedProject_WriteIsVisibleToALaterReader(t *testing.T) {
	dir := protoDir(t)
	shared := NewSharedProject(dir)
	defer func() { _ = shared.Close() }()

	writer := NewAPIClient(shared)
	defer func() { _ = writer.Close() }()

	seed := []docs.SeedOp{seedTicket("TKT-shared", "Before")}
	if _, err := writer.Do(context.Background(), docs.APIRequest{
		ProjectDir: dir, Seed: seed,
		Path: "/api/v1/tickets/TKT-shared", As: "editor",
	}); err != nil {
		t.Fatalf("priming: %v", err)
	}

	// A real write through the API, the way an api{} island makes one.
	resp, err := writer.Do(context.Background(), docs.APIRequest{
		ProjectDir: dir, Seed: seed, Method: "PATCH",
		Path: "/api/v1/tickets/TKT-shared", As: "editor",
		Body: `{"properties":{"title":"After"}}`,
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if resp.Status < 200 || resp.Status >= 300 {
		t.Fatalf("write status = %d, body = %s", resp.Status, resp.Body)
	}

	// A SECOND consumer of the same shared project — standing in for the
	// capturer, which reaches the identical *project through acquire.
	proj, err := shared.acquire(context.Background(), dir, seed, false)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	reader := NewAPIClient(shared)
	defer func() { _ = reader.Close() }()
	got, err := reader.Do(context.Background(), docs.APIRequest{
		ProjectDir: dir, Seed: seed,
		Path: "/api/v1/tickets/TKT-shared", As: "editor",
	})
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(got.Body, "After") {
		t.Errorf("the later reader does not see the earlier write; body = %s", got.Body)
	}
	if proj.server == nil {
		t.Error("acquire returned a project with no server")
	}
}

// Both consumers must get the SAME *project — not two that happen to be seeded
// alike. Pointer identity is the property; equal contents would still leave two
// stores diverging on the first write.
func TestSharedProject_BothConsumersGetOneProject(t *testing.T) {
	dir := protoDir(t)
	shared := NewSharedProject(dir)
	defer func() { _ = shared.Close() }()

	ctx := context.Background()
	seed := []docs.SeedOp{seedTicket("TICKET-ident", "One")}

	a, err := shared.acquire(ctx, dir, seed, false)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	b, err := shared.acquire(ctx, dir, seed, false)
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}
	if a != b {
		t.Fatal("the two consumers got different projects — the whole change is that they share one")
	}
}

// Documents must stay isolated: two manuals built in the same process get
// different holders, and a fixture from one must never appear in the other's
// figures. A leak here would be order-dependent, which is the worst shape of
// bug to debug.
func TestSharedProject_DocumentsAreIsolated(t *testing.T) {
	dir := protoDir(t)
	ctx := context.Background()

	docA := NewSharedProject(dir)
	defer func() { _ = docA.Close() }()
	docB := NewSharedProject(dir)
	defer func() { _ = docB.Close() }()

	clientA := NewAPIClient(docA)
	defer func() { _ = clientA.Close() }()
	clientB := NewAPIClient(docB)
	defer func() { _ = clientB.Close() }()

	seedA := []docs.SeedOp{seedTicket("TICKET-onlyA", "Only in A")}
	if _, err := clientA.Do(ctx, docs.APIRequest{
		ProjectDir: dir, Seed: seedA,
		Path: "/api/v1/tickets/TICKET-onlyA", As: "editor",
	}); err != nil {
		t.Fatalf("doc A: %v", err)
	}

	// Document B never seeded TICKET-onlyA, so it must 404 there.
	resp, err := clientB.Do(ctx, docs.APIRequest{
		ProjectDir: dir, Seed: nil,
		Path: "/api/v1/tickets/TICKET-onlyA", As: "editor",
	})
	if err != nil {
		t.Fatalf("doc B: %v", err)
	}
	if resp.Status != 404 {
		t.Errorf("document B can see document A's fixture (status %d) — documents must be isolated", resp.Status)
	}
}

// The seed applies ONCE and later ops accumulate: nothing is replayed, which is
// what lets a real write survive into every later island instead of being
// clobbered by re-application of the fixture.
func TestSharedProject_SeedAppliesOnceAndAccumulates(t *testing.T) {
	dir := protoDir(t)
	shared := NewSharedProject(dir)
	defer func() { _ = shared.Close() }()
	ctx := context.Background()

	first := []docs.SeedOp{seedTicket("TICKET-a1", "A1")}
	p, err := shared.acquire(ctx, dir, first, false)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if p.seeded != len(first) {
		t.Fatalf("watermark = %d, want %d", p.seeded, len(first))
	}

	grown := append(append([]docs.SeedOp{}, first...), seedTicket("TICKET-a2", "A2"))
	if _, err := shared.acquire(ctx, dir, grown, false); err != nil {
		t.Fatalf("grown: %v", err)
	}
	if p.seeded != len(grown) {
		t.Errorf("watermark = %d after growth, want %d", p.seeded, len(grown))
	}

	// Re-acquiring with the SAME seed must not re-apply anything (a re-applied
	// create would collide with the existing row).
	if _, err := shared.acquire(ctx, dir, grown, false); err != nil {
		t.Fatalf("idempotent re-acquire failed, so the seed was replayed: %v", err)
	}
}

// Close must be safe to call twice and must refuse later use, so the deferred
// teardown on a FAILED build cannot be undone by a straggling island.
func TestSharedProject_CloseIsIdempotentAndRefusesLaterUse(t *testing.T) {
	dir := protoDir(t)
	shared := NewSharedProject(dir)
	ctx := context.Background()

	if _, err := shared.acquire(ctx, dir, nil, false); err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := shared.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := shared.Close(); err != nil {
		t.Fatalf("second Close must be a no-op, got: %v", err)
	}
	if _, err := shared.acquire(ctx, dir, nil, false); err == nil {
		t.Error("acquire after Close must fail — otherwise it silently stands up a leaked project")
	}
}
