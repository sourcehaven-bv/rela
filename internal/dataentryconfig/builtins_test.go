package dataentryconfig

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidateBuiltins(t *testing.T) {
	meta := testMetamodel()
	lists := map[string]List{"tickets": {EntityType: "ticket"}}
	nav := []NavigationEntry{{Label: "Tickets", List: "tickets"}}
	space := func(b Builtins) []Space {
		return []Space{{ID: "crm", Label: "CRM", Navigation: nav, Builtins: b}}
	}

	cases := []struct {
		name     string
		builtins Builtins
		spaces   []Space
		wantErr  string // substring; "" means expect success
	}{
		{"no block", nil, nil, ""},
		{"every row, top level", Builtins{
			BuiltinSearch: {Hidden: true}, BuiltinInbox: {Hidden: true}, BuiltinFlowTasks: {Hidden: true},
			BuiltinFlowRuns: {Permission: "staff:use"}, BuiltinAnalysis: {Permission: "staff:use"},
		}, nil, ""},
		{"in a space", nil, space(Builtins{BuiltinSearch: {Hidden: true}}), ""},
		{"settings and about in a space", nil,
			space(Builtins{BuiltinSettings: {Permission: "settings:edit"}, BuiltinAbout: {Hidden: true}}), ""},
		{
			"unknown row",
			Builtins{"serach": {Hidden: true}},
			nil,
			`builtins: unknown built-in row "serach" (valid rows: search, inbox, flow_tasks, flow_runs, analysis, as_of, shortcuts, settings, about)`,
		},
		{
			"unknown row in a space",
			nil,
			space(Builtins{"setings": {Hidden: true}}),
			`spaces[crm].builtins: unknown built-in row "setings"`,
		},
		{"hidden and permission", Builtins{BuiltinSearch: {Hidden: true, Permission: "p"}}, nil,
			"builtins.search: set hidden or permission, not both"},
		{"neither set", Builtins{BuiltinAnalysis: {}}, nil,
			"builtins.analysis: set hidden: true, hidden_on_phone: true or a permission"},
		{"blank permission", Builtins{BuiltinAnalysis: {Permission: "  "}}, nil,
			"builtins.analysis: set hidden: true, hidden_on_phone: true or a permission"},
		{"hidden with a blank permission", Builtins{BuiltinSearch: {Hidden: true, Permission: "  "}}, nil,
			`builtins.search: permission "  " has leading or trailing spaces`},
		{"hidden on phone", Builtins{BuiltinShortcuts: {HiddenOnPhone: true}}, nil, ""},
		{"hidden on phone and a permission",
			Builtins{BuiltinShortcuts: {HiddenOnPhone: true, Permission: "staff:use"}}, nil, ""},
		{"hidden and hidden on phone", Builtins{BuiltinShortcuts: {Hidden: true, HiddenOnPhone: true}}, nil,
			"builtins.shortcuts: set hidden or hidden_on_phone, not both"},
		{"shortcuts hidden", Builtins{BuiltinShortcuts: {Hidden: true}}, nil, ""},
		{"padded permission", Builtins{BuiltinAnalysis: {Permission: " p"}}, nil,
			`builtins.analysis: permission " p" has leading or trailing spaces`},
		{
			"top level with spaces",
			Builtins{BuiltinSearch: {Hidden: true}},
			space(nil),
			"builtins and spaces are both set: move the top-level builtins into each space",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Lists: lists, Builtins: tc.builtins, Spaces: tc.spaces}
			if tc.spaces == nil {
				cfg.Navigation = nav
			}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, meta)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("expected success, got error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// `builtins:` is a known top-level key and a space key, and a misspelled field
// inside an entry leaves it empty, which validation refuses.
func TestBuiltins_ParseFromYAML(t *testing.T) {
	data := []byte(`version: "1.0"
lists:
  tickets: {entity_type: ticket}
spaces:
  - id: staff
    label: Staff
    builtins:
      analysis: {permission: staff:use}
  - id: portal
    label: Portal
    builtins:
      search: {hidden: true}
      flow_tasks: {hidden: true}
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(data, &cfg, testMetamodel()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got := cfg.Spaces[0].Builtins[BuiltinAnalysis].Permission; got != "staff:use" {
		t.Errorf("staff analysis permission = %q", got)
	}
	if !cfg.Spaces[1].Builtins[BuiltinSearch].Hidden || !cfg.Spaces[1].Builtins[BuiltinFlowTasks].Hidden {
		t.Errorf("portal builtins = %+v", cfg.Spaces[1].Builtins)
	}

	typo := []byte(`version: "1.0"
builtins:
  search: {hiden: true}
`)
	var bad Config
	if err := yaml.Unmarshal(typo, &bad); err != nil {
		t.Fatal(err)
	}
	err := ValidateConfig(typo, &bad, testMetamodel())
	if err == nil || !strings.Contains(err.Error(), "builtins.search: set hidden: true, hidden_on_phone: true or a permission") {
		t.Fatalf("a misspelled field must be refused, got %v", err)
	}
}

func TestBuiltins_Hidden(t *testing.T) {
	b := Builtins{
		BuiltinAnalysis: {Hidden: true},
		BuiltinSearch:   {Permission: "held"},
		BuiltinInbox:    {Permission: "missing"},
		BuiltinAsOf:     {Permission: "missing"},
	}
	got := b.Hidden(func(p string) bool { return p == "held" })
	want := []BuiltinRow{BuiltinInbox, BuiltinAnalysis, BuiltinAsOf}
	if !slices.Equal(got, want) {
		t.Errorf("Hidden = %v, want %v (sidebar order)", got, want)
	}
	if got := Builtins(nil).Hidden(func(string) bool { return false }); got != nil {
		t.Errorf("nil Builtins hides %v", got)
	}
}

func TestBuiltins_HiddenOnPhone(t *testing.T) {
	cases := []struct {
		name string
		b    Builtins
		want []BuiltinRow
	}{
		{"no block", nil, nil},
		{"none set", Builtins{BuiltinSearch: {Hidden: true}}, nil},
		{"sidebar order", Builtins{
			BuiltinShortcuts: {HiddenOnPhone: true},
			BuiltinSearch:    {HiddenOnPhone: true, Permission: "p"},
			BuiltinAnalysis:  {Permission: "p"},
		}, []BuiltinRow{BuiltinSearch, BuiltinShortcuts}},
		{"hidden for everyone is left to Hidden", Builtins{
			BuiltinShortcuts: {Hidden: true, HiddenOnPhone: true},
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.b.HiddenOnPhone(); !slices.Equal(got, tc.want) {
				t.Errorf("HiddenOnPhone = %v, want %v", got, tc.want)
			}
		})
	}
}

// `hidden_on_phone:` decodes from YAML on the shortcuts entry.
func TestBuiltins_HiddenOnPhoneFromYAML(t *testing.T) {
	data := []byte(`version: "1.0"
lists:
  tickets: {entity_type: ticket}
navigation:
  - {label: Tickets, list: tickets}
builtins:
  shortcuts: {hidden_on_phone: true}
`)
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig(data, &cfg, testMetamodel()); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !cfg.Builtins[BuiltinShortcuts].HiddenOnPhone {
		t.Errorf("builtins = %+v", cfg.Builtins)
	}
}
