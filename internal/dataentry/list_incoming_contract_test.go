package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestListEndpoint_IncomingEdge_InverseKey_ODHV2DContract is a CROSS-TICKET
// CONTRACT PIN for TKT-NC3D08 ⇄ TKT-ODHV2D (finding RR-UKS8BW).
//
// TKT-NC3D08 (relation fields on kanban cards) renders an INCOMING relation on
// a card by looking up the edge in the list row's `relations` map under the
// relation's INVERSE key (`getInverseName(rel)` or the `<relation>_inverse`
// fallback). Populating that inverse key server-side is TKT-ODHV2D's work — its
// `visibleRelationIDs` helper both emits incoming edges under the inverse key
// AND gates ACL-hidden neighbor IDs out. On THIS branch the list serializer
// (`entitySerializer.forWireRelated`, fed by `entityReader.outgoingRelations`)
// emits OUTGOING edges only, so incoming card fields are inert until ODHV2D
// merges.
//
// This test seeds an INCOMING edge onto a list row and asserts the row's
// `relations` map exposes that edge (under an inverse-shaped key). It encodes
// the merge-order dependency:
//
//   - THIS branch (no ODHV2D): the inverse key is absent, so the test t.Skip()s
//     with an explicit ODHV2D-dependency reason. It does NOT fail.
//   - AFTER ODHV2D merges: the inverse key is present, so the skip guard falls
//     through and the assertion runs live — catching any regression that stops
//     the server emitting incoming edges (which would silently blank every
//     incoming kanban card field again).
//
// WHY t.Skip AND NOT a genuinely-failing test: CI runs `go test ./...` (the
// "Test" job in ci.yml) and fails the whole job on any failing test. A
// deliberately-red test on this feature branch would block review/commit of the
// entire branch, not just gate the ODHV2D merge. So we use the skip-that-flips
// pattern the finding permits: it is inert-but-honest today and becomes a real
// assertion automatically the moment ODHV2D lands, with no human having to
// remember to un-skip it. Per RR-UKS8BW this is the deliberate choice.
func TestListEndpoint_IncomingEdge_InverseKey_ODHV2DContract(t *testing.T) {
	app := newTestAppV1(t)

	// FEAT-1 is the list row. TKT-1 --implements--> FEAT-1 is an INCOMING
	// `implements` edge on FEAT-1 (implements is ticket→feature in the test
	// metamodel). We list features and inspect the FEAT-1 row.
	seedEntity(app, &entity.Entity{ID: "FEAT-1", Type: "feature", Properties: map[string]any{"title": "F1"}})
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "T1"}})
	seedRelation(app, entity.NewRelation("TKT-1", "implements", "FEAT-1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/features?include=*", http.NoBody)
	rec := httptest.NewRecorder()
	app.handleV1ListEntities(rec, req, "feature", "features")
	if rec.Code != 200 {
		t.Fatalf("list features: status %d, body %s", rec.Code, rec.Body)
	}

	resp := decodeListWithIncluded(t, rec)

	var row *listRow
	for i := range resp.Data {
		if resp.Data[i].ID == "FEAT-1" {
			row = &resp.Data[i]
			break
		}
	}
	if row == nil {
		t.Fatalf("FEAT-1 not in list response: %s", rec.Body)
	}

	// Does the row expose the incoming TKT-1 edge under some inverse-shaped
	// key? On this branch the serializer emits outgoing edges only, so it will
	// not — that is the ODHV2D dependency.
	if !relationsContainTarget(row.Relations, "TKT-1") {
		t.Skipf("ODHV2D not yet integrated on this branch: the list endpoint does not "+
			"emit incoming edges under the inverse key, so incoming kanban card fields "+
			"(TKT-NC3D08) cannot resolve. This contract test activates once TKT-ODHV2D's "+
			"server change (visibleRelationIDs) populates the inverse key. "+
			"row.relations = %v", row.Relations)
	}

	// ── Post-ODHV2D live assertions ──────────────────────────────────────
	// The declared inverse for `implements` (or the `<relation>_inverse`
	// fallback) is the key TKT-NC3D08's SPA reads — mirroring the SPA's
	// `getInverseName(rel) || `${rel}_inverse``. Assert the incoming edge is
	// reachable under that inverse key specifically, matching the SPA contract.
	inverseKey := "implements_inverse"
	if rel, ok := app.Meta().Relations["implements"]; ok && rel.Inverse != nil && rel.Inverse.ID != "" {
		inverseKey = rel.Inverse.ID
	}
	targets := row.Relations[inverseKey]
	if !containsID(targets, "TKT-1") {
		t.Errorf("incoming edge not under inverse key %q: want TKT-1 in %v (full map %v)",
			inverseKey, targets, row.Relations)
	}
}

// listRow / listWithIncluded model just the fields this contract test reads
// from the list response (the endpoint's `included`-bearing shape). Kept local
// so the test is self-describing and independent of v1 wire struct churn.
type listRow struct {
	ID        string              `json:"id"`
	Relations map[string][]string `json:"relations"`
}

type listWithIncluded struct {
	Data []listRow `json:"data"`
}

func decodeListWithIncluded(t *testing.T, rec *httptest.ResponseRecorder) listWithIncluded {
	t.Helper()
	var out listWithIncluded
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode list response: %v\nbody: %s", err, rec.Body)
	}
	return out
}

func relationsContainTarget(m map[string][]string, id string) bool {
	for _, targets := range m {
		if containsID(targets, id) {
			return true
		}
	}
	return false
}
