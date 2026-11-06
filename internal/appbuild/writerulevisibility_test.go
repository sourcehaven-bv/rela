package appbuild_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const writeRuleVisibilityMetamodel = `version: "1.0"
entities:
  budget:
    label: Budget
    id_prefix: "BUD-"
    id_type: sequential
    properties:
      name: {type: string}
      limit: {type: integer}
      amount: {type: integer}
relations: {}
validations:
  - name: binnen-limiet
    entity_type: budget
    enforce: write
    then_condition: "entity.amount == nil or entity.limit == nil or entity.amount <= entity.limit"
  - name: naam-verplicht
    entity_type: budget
    enforce: write
    then: ["name!="]
  - name: alleen-rapport
    entity_type: budget
    then: ["amount!="]
`

func TestWriteRuleVisibilityConflicts(t *testing.T) {
	meta, err := metamodel.Parse([]byte(writeRuleVisibilityMetamodel))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		policy string
		want   []string // substrings, one conflict each
	}{
		{
			name: "rule reads hidden properties",
			policy: `roles:
  invoerder:
    read: ["*"]
    visible:
      budget:
        - field: name
        - field: limit
assignments:
  bob: invoerder
`,
			want: []string{`validation "binnen-limiet" (enforce: write) reads "amount", which role "invoerder"`},
		},
		{
			name: "no visible restriction",
			policy: `roles:
  invoerder:
    read: ["*"]
assignments:
  bob: invoerder
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := acl.LoadPolicyBytes([]byte(tc.policy))
			if err != nil {
				t.Fatal(err)
			}
			got := appbuild.WriteRuleVisibilityConflicts(meta, pol)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d conflicts %v, want %d", len(got), got, len(tc.want))
			}
			for i, w := range tc.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("conflict %d = %q, want it to contain %q", i, got[i], w)
				}
			}
		})
	}
	if got := appbuild.WriteRuleVisibilityConflicts(nil, nil); got != nil {
		t.Errorf("nil inputs = %v, want nil", got)
	}
}
