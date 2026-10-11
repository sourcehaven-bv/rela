package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// pilesMeta is testMetamodel with one registered transform.
func pilesMeta() *metamodel.Metamodel {
	m := testMetamodel()
	m.Transforms = map[string]metamodel.TransformDef{
		"docx": {From: "markdown", Command: []string{"pandoc", "{in}", "-o", "{out}"}, Produces: "docx"},
	}
	return m
}

// pilesActions declares the actions the piles cases refer to.
const pilesActions = `actions:
  close: { label: Close, set: { status: closed } }
  unlabelled: { set: { status: closed } }
  bound:
    label: Bound
    script: bound.lua
    available_on: { entity_types: [ticket] }
  ghost: { label: Ghost, set: { nowhere: x } }
`

func TestValidateConfig_Piles(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string // substring; "" means expect success
	}{
		{"absent", ``, ""},
		{"empty block", "piles: {}", ""},
		{"actions and export", "piles: {actions: [close], export: [docx]}", ""},

		{"unknown action", "piles: {actions: [nope]}", `piles.actions: unknown action "nope"`},
		{"entity-bound action", "piles: {actions: [bound]}", `action "bound" is entity-bound`},
		{"unlabelled action", "piles: {actions: [unlabelled]}", `action "unlabelled" needs a label`},
		{"unknown transform", "piles: {export: [pdf]}", `piles.export: unknown transform "pdf"`},
		{"unknown key", "piles: {action: [close]}", `piles: unknown key "action"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("version: \"1.0\"\n" + pilesActions + tc.yaml)
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			err := ValidateConfig(data, &cfg, pilesMeta())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestCollectConfigWarnings_PilesSetAction pins that a `set:` action whose
// property no type declares is a warning, not an error: it is offered, and
// fails per item.
func TestCollectConfigWarnings_PilesSetAction(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string // substring; "" means no piles warning
	}{
		{"declared property", "piles: {actions: [close]}", ""},
		{"undeclared property", "piles: {actions: [ghost]}", `sets property "nowhere", which no entity type declares`},
		{"no block", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("version: \"1.0\"\n" + pilesActions + tc.yaml)
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			var got []string
			for _, w := range CollectConfigWarnings(&cfg, pilesMeta()) {
				if strings.HasPrefix(w, "piles.") {
					got = append(got, w)
				}
			}
			if tc.want == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected piles warnings: %v", got)
				}
				return
			}
			if len(got) != 1 || !strings.Contains(got[0], tc.want) {
				t.Fatalf("warnings = %v, want one containing %q", got, tc.want)
			}
		})
	}
}
