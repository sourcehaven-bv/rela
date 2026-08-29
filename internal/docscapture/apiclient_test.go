package docscapture

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// The API client serves a real request against a seeded temp project, with no
// browser and no built frontend — the property that lets api{} gate CI.
func TestAPIClient_ServesSeededEntity(t *testing.T) {
	dir := protoDir(t)
	c := NewAPIClient(dir)
	defer func() { _ = c.Close() }()

	seed := []docs.SeedOp{{
		Kind: "create", Type: "ticket", ID: "TICKET-api",
		Properties: map[string]any{"title": "Seeded", "status": "open", "priority": "low", "reporter": "a@b.c"},
	}}

	resp, err := c.Do(context.Background(), docs.APIRequest{
		ProjectDir: dir, Seed: seed,
		Path: "/api/v1/tickets/TICKET-api", As: "editor",
	})
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("status = %d, body = %s", resp.Status, resp.Body)
	}
	if !strings.Contains(resp.Body, "Seeded") {
		t.Errorf("body does not carry the seeded entity: %s", resp.Body)
	}
}

// An HTTP error status is a RESPONSE, not a transport error — asserting a 404
// is the point of the verb, so it must not surface as err.
func TestAPIClient_ErrorStatusIsNotAnError(t *testing.T) {
	dir := protoDir(t)
	c := NewAPIClient(dir)
	defer func() { _ = c.Close() }()

	resp, err := c.Do(context.Background(), docs.APIRequest{
		ProjectDir: dir, Path: "/api/v1/tickets/NOPE", As: "editor",
	})
	if err != nil {
		t.Fatalf("a 404 must be a response, not an error: %v", err)
	}
	if resp.Status != 404 {
		t.Fatalf("status = %d, body = %s", resp.Status, resp.Body)
	}
}

// The seed grows as a manual runs; a later request must see entities created
// after the server stood up (the syncSeed path screenshot{} also relies on).
func TestAPIClient_SeedGrowsAcrossRequests(t *testing.T) {
	dir := protoDir(t)
	c := NewAPIClient(dir)
	defer func() { _ = c.Close() }()

	first := []docs.SeedOp{{
		Kind: "create", Type: "ticket", ID: "TICKET-one",
		Properties: map[string]any{"title": "One", "status": "open", "priority": "low", "reporter": "a@b.c"},
	}}
	if _, err := c.Do(context.Background(), docs.APIRequest{
		ProjectDir: dir, Seed: first, Path: "/api/v1/tickets/TICKET-one", As: "editor",
	}); err != nil {
		t.Fatalf("first: %v", err)
	}

	grown := append(append([]docs.SeedOp{}, first...), docs.SeedOp{
		Kind: "create", Type: "ticket", ID: "TICKET-two",
		Properties: map[string]any{"title": "Two", "status": "open", "priority": "low", "reporter": "a@b.c"},
	})
	resp, err := c.Do(context.Background(), docs.APIRequest{
		ProjectDir: dir, Seed: grown, Path: "/api/v1/tickets/TICKET-two", As: "editor",
	})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if resp.Status != 200 {
		t.Fatalf("an entity seeded after standUp was not served: %d %s", resp.Status, resp.Body)
	}
}
