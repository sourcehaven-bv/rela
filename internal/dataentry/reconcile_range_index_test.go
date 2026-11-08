package dataentry

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/reconwire"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
)

// TestReconcile_PlanErrorNamesRange: a planning error about one range
// names that range with the JSON pointer /ranges/<index>; one about the
// request as a whole names none (TKT-0Y566OJ).
func TestReconcile_PlanErrorNamesRange(t *testing.T) {
	f := newReconcileFixture(t, reconcileGateConfig{})
	cases := []struct {
		name string
		body reconwire.Request
		want *int
	}{
		{"second range names an unknown type",
			reconwire.Request{Ranges: []reconwire.Range{fpRange("ticket"), fpRange("nope")}}, new(int)},
		{"no ranges", reconwire.Request{}, nil},
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
			case tc.want == nil && len(p.Errors) != 0:
				t.Errorf("errors = %+v, want none", p.Errors)
			case tc.want != nil && (len(p.Errors) != 1 || p.Errors[0].Source.Pointer != "/ranges/"+strconv.Itoa(*tc.want)):
				t.Errorf("errors = %+v, want one at /ranges/%d", p.Errors, *tc.want)
			}
		})
	}
}
