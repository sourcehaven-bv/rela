package appbuild_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
)

// traversalFieldSchema scopes a feature on the status and the owner of the
// tickets implementing it, walking the relation backwards.
const traversalFieldSchema = `version: "1.0"
entities:
  feature:
    label: Feature
    plural: features
    id_prefix: "FEAT-"
    id_type: sequential
    properties:
      title:
        type: string
    query_scopes:
      busy: "related(entity, 'implementedBy', { status = 'in-progress' })"
      mine: "related(entity, 'implementedBy', { owner = 'bob' })"
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      status:
        type: string
      owner:
        type: string
relations:
  implements:
    label: implements
    inverse: implementedBy
    from: [ticket]
    to: [feature]
`

func TestQueryScopeTraversalFieldErrors(t *testing.T) {
	cases := []struct {
		name   string
		policy string
		want   []string // substrings; empty means no errors
	}{
		{name: "no policy"},
		{
			name: "every role sees both fields",
			policy: `roles:
  viewer:
    read: ["*"]
assignments:
  bob: viewer
`,
		},
		{
			name: "closed-world visible list hides owner",
			policy: `roles:
  viewer:
    read: ["*"]
    visible:
      ticket:
        - field: status
assignments:
  bob: viewer
`,
			want: []string{`query scope "mine"`, `"owner"`, `"ticket"`},
		},
		{
			name: "conditional grant on status",
			policy: `roles:
  viewer:
    read: ["*"]
    visible:
      ticket:
        - field: status
          when: "entity.owner == 'bob'"
        - field: owner
assignments:
  bob: viewer
`,
			want: []string{`query scope "busy"`, `"status"`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			compiled, meta, pol := mustScopeFixture(t, traversalFieldSchema, tc.policy)
			got := appbuild.QueryScopeTraversalFieldErrors(compiled, meta, pol)
			if len(tc.want) == 0 {
				if len(got) != 0 {
					t.Fatalf("want no errors, got %v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("want 1 error, got %d: %v", len(got), got)
			}
			for _, w := range tc.want {
				if !strings.Contains(got[0], w) {
					t.Errorf("error %q missing %q", got[0], w)
				}
			}
		})
	}
}
