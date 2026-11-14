package actionproposal

import (
	"encoding/json"
	"testing"
)

// The JSON shape is what the SPA dialog and MCP callers read.
func TestProposal_JSON(t *testing.T) {
	tests := []struct {
		name string
		p    *Proposal
		want string
	}{
		{
			name: "empty lists encode as []",
			p:    Empty("TKT-1"),
			want: `{"entity":"TKT-1","set":[],"relate":[]}`,
		},
		{
			name: "unset current encodes as null",
			p: &Proposal{
				Entity: "TKT-1",
				Set:    []Value{{Property: "priority", Proposed: json.RawMessage(`"high"`)}},
				Relate: []Relation{{Relation: "affects", To: "CON-1", ToType: "concept", ToTitle: "Sessions"}},
				Model:  "m",
			},
			want: `{"entity":"TKT-1","set":[{"property":"priority","current":null,"proposed":"high"}],` +
				`"relate":[{"relation":"affects","to":"CON-1","to_type":"concept","to_title":"Sessions"}],"model":"m"}`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.p)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
}
