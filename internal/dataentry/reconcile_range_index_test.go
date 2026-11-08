package dataentry

import (
	"encoding/json"
	"net/http"
	"testing"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
)

// TestReconcile_PlanErrorNamesRange: a planning error about one range
// carries that range's index; one about the request as a whole carries
// none (TKT-0Y566OJ).
func TestReconcile_PlanErrorNamesRange(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	cases := []struct {
		name string
		body v1.ReconcileRequest
		want *int
	}{
		{"second range names an unknown type",
			v1.ReconcileRequest{Ranges: []v1.ReconcileRange{fpRange("ticket"), fpRange("nope")}}, new(int)},
		{"no ranges", v1.ReconcileRequest{}, nil},
	}
	*cases[0].want = 1
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := f.post("alice", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			var p v1.Error
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			switch {
			case tc.want == nil && p.Range != nil:
				t.Errorf("range = %d, want none", *p.Range)
			case tc.want != nil && (p.Range == nil || *p.Range != *tc.want):
				t.Errorf("range = %v, want %d", p.Range, *tc.want)
			}
		})
	}
}
