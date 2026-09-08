package dataentry

import (
	"encoding/json"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestList_RowsCarryFaceProvenance is the BUG-3 prerequisite: a list row must
// say WHICH FACE it is, or nothing downstream can label it.
//
// The relation picker (and any list) offers rows that may be any face in the
// world's chain. A first-choice hit and a within-chain fallback arrive
// byte-identically — same id, same title — so `_world` is the only thing that
// separates "this IS the published policy" from "this is the draft, standing
// in because nothing is published". Without it a badge would be guesswork.
func TestList_RowsCarryFaceProvenance(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-P", Type: "ticket", Properties: map[string]any{"title": "draft"},
	})
	if err := app.store.CreateEntity(t.Context(), &entity.Entity{
		ID: "TKT-P", Type: "ticket", Face: entity.Face("published"),
		Properties: map[string]any{"title": "published"},
	}); err != nil {
		t.Fatalf("seed published face: %v", err)
	}
	app.SetWorlds(stubWorlds{names: map[string]bool{"published": true}})

	rows := listRows(t, app, "/api/v1/tickets?world=published")
	if len(rows) != 1 {
		t.Fatalf("want one row; got %d", len(rows))
	}
	w, ok := rows[0]["_world"].(map[string]any)
	if !ok {
		t.Fatalf("a row under a world must carry `_world` provenance, or a "+
			"picker cannot tell which face it is offering; got %v", rows[0]["_world"])
	}
	if w["face"] != "published" {
		t.Errorf("_world.face = %v, want published", w["face"])
	}
	if w["via"] != ruleChain {
		t.Errorf("_world.via = %v, want %q", w["via"], ruleChain)
	}
	// chain_position is what separates a first-choice hit from a within-chain
	// fallback; the badge keys on it, so a row that omits it is unlabelable.
	if pos, present := w["chain_position"]; !present || pos != float64(0) {
		t.Errorf("_world.chain_position = %v (present=%v), want 0 — the badge "+
			"keys on this to tell a prime from a stand-in", pos, present)
	}
}

// TestList_DefaultWorldRowsCarryNoProvenance pins the other half: in the
// default world every row IS the default face, so a block on every row would
// be noise — and worse, would imply a world was applied when none was. This is
// the same choice the view collections make.
func TestList_DefaultWorldRowsCarryNoProvenance(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-D", Type: "ticket", Properties: map[string]any{"title": "draft"},
	})

	rows := listRows(t, app, "/api/v1/tickets")
	if len(rows) != 1 {
		t.Fatalf("want one row; got %d", len(rows))
	}
	if got, present := rows[0]["_world"]; present {
		t.Errorf("a default-world row must carry NO `_world` — every row is the "+
			"default face there, so a block on each would be noise implying a "+
			"world was applied; got %v", got)
	}
}

func listRows(t *testing.T, app *App, path string) []map[string]any {
	t.Helper()
	rec := viewRecord(t, app, path)
	if rec.Code != 200 {
		t.Fatalf("%s: got %d %s", path, rec.Code, rec.Body)
	}
	var resp struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Data
}
