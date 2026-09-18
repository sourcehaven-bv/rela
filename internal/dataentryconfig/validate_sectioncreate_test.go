package dataentryconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// sectionCreateConfig builds a config whose single view has one relation
// section, with the given create block applied to it.
//
// The view follows `belongs-to` (ticket -> category, one reachable type) unless
// a test overrides the traverse rule.
func sectionCreateConfig(create *SectionCreate, traverse ...ViewTraverse) *Config {
	if len(traverse) == 0 {
		traverse = []ViewTraverse{
			{From: "entry", Follow: "belongs-to", CollectAs: "cats"},
		}
	}
	return &Config{
		Version: "1.0",
		App:     AppConfig{Name: "Test App"},
		Forms: map[string]Form{
			"new_category": {EntityType: "category", Title: "New Category"},
			"new_ticket":   {EntityType: "ticket", Title: "New Ticket"},
		},
		Views: map[string]ViewConfig{
			"ticket_detail": {
				Entry:    ViewEntry{Type: "ticket"},
				Traverse: traverse,
				Sections: []ViewSection{
					{Heading: "Categories", Source: "cats", Display: "cards", Create: create},
				},
			},
		},
	}
}

// validateOneView runs the view validator and returns its errors joined, so a
// test can assert on substrings without indexing into a slice whose order is
// not contractual.
func validateOneView(t *testing.T, cfg *Config) string {
	t.Helper()
	return strings.Join(validateViews(cfg, testMetamodel()), "\n")
}

// TestSectionCreate_OptInIsRequired pins TKT-651W's invariant as narrowed by
// TKT-R4BMJM: a section says nothing unless it opts in. This is the DEFAULT, so
// it is the case most likely to be broken by a later "helpful" change.
func TestSectionCreate_OptInIsRequired(t *testing.T) {
	t.Run("absent create block is valid and yields no affordance", func(t *testing.T) {
		cfg := sectionCreateConfig(nil)
		if got := validateOneView(t, cfg); got != "" {
			t.Fatalf("a section without create must load cleanly, got: %s", got)
		}
		sec := cfg.Views["ticket_detail"].Sections[0]
		if sec.Create.PlacedIn(SectionCreateInSection) {
			t.Error("a nil Create must be placed nowhere")
		}
		if sec.Create.PlacedIn(SectionCreateInHeader) {
			t.Error("a nil Create must not reach the header")
		}
	})

	t.Run("empty create block opts in with defaults", func(t *testing.T) {
		cfg := sectionCreateConfig(&SectionCreate{})
		if got := validateOneView(t, cfg); got != "" {
			t.Fatalf("create: {} must load cleanly, got: %s", got)
		}
		c := cfg.Views["ticket_detail"].Sections[0].Create
		if got := c.EffectiveFlow(); got != SectionCreateFlowModal {
			t.Errorf("default flow = %q, want %q", got, SectionCreateFlowModal)
		}
		if !c.PlacedIn(SectionCreateInSection) {
			t.Error("default placement must include the section")
		}
		if c.PlacedIn(SectionCreateInHeader) {
			t.Error("default placement must NOT include the header — that is opt-in too")
		}
	})
}

// TestSectionCreate_UnmarshalRejectsScalars pins the "one spelling per meaning"
// rule. `create: false` is the dangerous case: without the custom unmarshal it
// decodes into a zero-valued struct and ENABLES the affordance, the exact
// opposite of what the operator wrote.
func TestSectionCreate_UnmarshalRejectsScalars(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string // "" means the value must be accepted
	}{
		{name: "mapping", yaml: "create: {flow: page}"},
		{name: "empty mapping", yaml: "create: {}"},
		{name: "false", yaml: "create: false", want: "must be a mapping"},
		{name: "true", yaml: "create: true", want: "must be a mapping"},
		{name: "string", yaml: "create: yes-please", want: "must be a mapping"},
		{name: "number", yaml: "create: 1", want: "must be a mapping"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var sec ViewSection
			err := yaml.Unmarshal([]byte(tc.yaml), &sec)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if sec.Create == nil {
					t.Fatal("a mapping must produce a non-nil Create")
				}
				return
			}
			if err == nil {
				t.Fatalf("%s was accepted; it must be refused so the operator "+
					"learns absence is how a section opts out", tc.yaml)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestSectionCreate_ValidationRejectsBadValues(t *testing.T) {
	tests := []struct {
		name   string
		create *SectionCreate
		want   string
	}{
		{
			name:   "unknown flow",
			create: &SectionCreate{Flow: "dialog"},
			want:   `create.flow "dialog" is invalid`,
		},
		{
			name:   "unknown placement",
			create: &SectionCreate{In: []string{"sidebar"}},
			want:   `create.in contains "sidebar"`,
		},
		{
			name:   "unreachable type override",
			create: &SectionCreate{Types: map[string]SectionCreateTarget{"ticket": {Template: "x"}}},
			want:   `create.types names "ticket", which relation "belongs-to" cannot reach`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := validateOneView(t, sectionCreateConfig(tc.create))
			if !strings.Contains(got, tc.want) {
				t.Errorf("errors = %q, want one containing %q", got, tc.want)
			}
		})
	}
}

// TestSectionCreate_RejectsTypeWithNoForm covers the gap between load-time
// validation and the runtime derivation: a type with no create form is dropped
// by the resolver, so without this check the config loads and then renders
// nothing, with no way to tell why.
func TestSectionCreate_RejectsTypeWithNoForm(t *testing.T) {
	cfg := sectionCreateConfig(&SectionCreate{})
	delete(cfg.Forms, "new_category") // leave the relation reaching only formless types

	got := validateOneView(t, cfg)
	if !strings.Contains(got, "no type reachable by relation") {
		t.Errorf("errors = %q, want one reporting that nothing reachable is creatable", got)
	}
}

// TestSectionCreate_RejectsSectionWithNoSingleRelation covers every shape where
// the originating relation is ambiguous. Each must be refused at load, because
// guessing a relation would link the new entity to the wrong peer — worse than
// no button at all.
func TestSectionCreate_RejectsSectionWithNoSingleRelation(t *testing.T) {
	tests := []struct {
		name     string
		traverse []ViewTraverse
		source   string
	}{
		{
			name:     "recursive rule",
			traverse: []ViewTraverse{{From: "entry", Follow: "belongs-to", CollectAs: "cats", Recursive: true}},
			source:   "cats",
		},
		{
			name: "rule collected from another bucket, not entry",
			traverse: []ViewTraverse{
				{From: "entry", Follow: "belongs-to", CollectAs: "cats"},
				{From: "cats", Follow: "relates-to", CollectAs: "deep"},
			},
			source: "deep",
		},
		{
			name:     "source is the entry itself",
			traverse: []ViewTraverse{{From: "entry", Follow: "belongs-to", CollectAs: "cats"}},
			source:   "entry",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := sectionCreateConfig(&SectionCreate{}, tc.traverse...)
			secs := cfg.Views["ticket_detail"].Sections
			secs[0].Source = tc.source
			cfg.Views["ticket_detail"] = ViewConfig{
				Entry:    ViewEntry{Type: "ticket"},
				Traverse: tc.traverse,
				Sections: secs,
			}

			got := validateOneView(t, cfg)
			if !strings.Contains(got, "no single relation fills it") {
				t.Errorf("errors = %q, want one refusing create on an ambiguous section", got)
			}
		})
	}
}

// TestSectionOriginRelation covers the shared resolver directly, including the
// link direction — which decides whether the new entity becomes the FROM or the
// TO of the created edge, and is therefore the difference between a correct
// link and a backwards one.
func TestSectionOriginRelation(t *testing.T) {
	tests := []struct {
		name         string
		traverse     ViewTraverse
		source       string
		wantRelation string
		wantLinkAs   string
		wantOK       bool
	}{
		{
			name:         "outgoing follow makes the new entity the TO",
			traverse:     ViewTraverse{From: "entry", Follow: "belongs-to", CollectAs: "cats"},
			source:       "cats",
			wantRelation: "belongs-to",
			wantLinkAs:   "to",
			wantOK:       true,
		},
		{
			name:         "incoming follow makes the new entity the FROM",
			traverse:     ViewTraverse{From: "entry", FollowIncoming: "belongs-to", CollectAs: "tix"},
			source:       "tix",
			wantRelation: "belongs-to",
			wantLinkAs:   "from",
			wantOK:       true,
		},
		{
			name:     "recursive is not attributable",
			traverse: ViewTraverse{From: "entry", Follow: "belongs-to", CollectAs: "cats", Recursive: true},
			source:   "cats",
		},
		{
			name:     "source matching no rule",
			traverse: ViewTraverse{From: "entry", Follow: "belongs-to", CollectAs: "cats"},
			source:   "other",
		},
		{
			name:     "empty source",
			traverse: ViewTraverse{From: "entry", Follow: "belongs-to", CollectAs: "cats"},
			source:   "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			view := ViewConfig{
				Entry:    ViewEntry{Type: "ticket"},
				Traverse: []ViewTraverse{tc.traverse},
			}
			rel, linkAs, ok := SectionOriginRelation(view, ViewSection{Source: tc.source})
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if rel != tc.wantRelation {
				t.Errorf("relation = %q, want %q", rel, tc.wantRelation)
			}
			if linkAs != tc.wantLinkAs {
				t.Errorf("linkAs = %q, want %q", linkAs, tc.wantLinkAs)
			}
		})
	}
}

// TestSectionCreate_HeterogeneousRelationAcceptsPerTypeTemplates is the case the
// per-type map exists for: one relation, several reachable types, a different
// template for each.
func TestSectionCreate_HeterogeneousRelationAcceptsPerTypeTemplates(t *testing.T) {
	create := &SectionCreate{
		Types: map[string]SectionCreateTarget{
			"ticket":   {Template: "bugfix"},
			"category": {Template: "grouping"},
		},
	}
	cfg := sectionCreateConfig(create,
		ViewTraverse{From: "entry", Follow: "relates-to", CollectAs: "cats"})

	if got := validateOneView(t, cfg); got != "" {
		t.Fatalf("per-type templates on a heterogeneous relation must load, got: %s", got)
	}
	if got := create.TemplateFor("ticket"); got != "bugfix" {
		t.Errorf("TemplateFor(ticket) = %q, want %q", got, "bugfix")
	}
	if got := create.TemplateFor("category"); got != "grouping" {
		t.Errorf("TemplateFor(category) = %q, want %q", got, "grouping")
	}
	// A type absent from the map still gets a button, just with no preset.
	if got := create.TemplateFor("unlisted"); got != "" {
		t.Errorf("TemplateFor(unlisted) = %q, want empty", got)
	}
}

// TestSectionCreate_SurvivesRoundTrips answers the question RR-19AU91 raised:
// DarkMode implements four methods (UnmarshalYAML, MarshalYAML, MarshalJSON,
// UnmarshalJSON), so does SectionCreate need the other three?
//
// It does not, and this test is why rather than an assertion of faith. The
// custom unmarshal only intercepts YAML DECODING to refuse a scalar; the struct
// tags already describe both encodings, so marshaling and JSON decoding need no
// help. ViewSection is json-tagged and ViewConfig is served over the wire, so a
// silent loss here would strip an operator's `create:` block from anything that
// re-serializes a view — hence a test rather than a code comment.
func TestSectionCreate_SurvivesRoundTrips(t *testing.T) {
	var sec ViewSection
	if err := yaml.Unmarshal(
		[]byte("source: tasks\ndisplay: cards\ncreate: {flow: page, in: [header]}"), &sec,
	); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}
	if sec.Create == nil || sec.Create.Flow != SectionCreateFlowPage {
		t.Fatalf("yaml decode produced %+v", sec.Create)
	}

	t.Run("json", func(t *testing.T) {
		b, err := json.Marshal(sec)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var back ViewSection
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if back.Create == nil {
			t.Fatal("the create block did not survive a JSON round-trip")
		}
		if back.Create.EffectiveFlow() != SectionCreateFlowPage {
			t.Errorf("flow = %q, want %q", back.Create.EffectiveFlow(), SectionCreateFlowPage)
		}
		if !back.Create.PlacedIn(SectionCreateInHeader) {
			t.Error("placement did not survive a JSON round-trip")
		}
	})

	t.Run("yaml", func(t *testing.T) {
		b, err := yaml.Marshal(sec)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var back ViewSection
		if err := yaml.Unmarshal(b, &back); err != nil {
			t.Fatalf("re-decode: %v", err)
		}
		if back.Create == nil {
			t.Fatal("the create block did not survive a YAML round-trip")
		}
		if back.Create.EffectiveFlow() != SectionCreateFlowPage {
			t.Errorf("flow = %q, want %q", back.Create.EffectiveFlow(), SectionCreateFlowPage)
		}
	})
}
