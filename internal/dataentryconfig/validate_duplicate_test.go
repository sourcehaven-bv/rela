package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// duplicateConfig builds a config whose entity_views carries the given
// duplicate block for `ticket`, with detail_view set unless cleared.
func duplicateConfig(dup *DuplicateConfig, detailView string) *Config {
	return &Config{
		Version: "1.0",
		App:     AppConfig{Name: "Test App"},
		Views: map[string]ViewConfig{
			"ticket_detail": {Entry: ViewEntry{Type: "ticket"}},
		},
		EntityViews: map[string]EntityViewConfig{
			"ticket": {DetailView: detailView, Duplicate: dup},
		},
	}
}

func TestValidateEntityViews_Duplicate(t *testing.T) {
	tests := []struct {
		name       string
		dup        *DuplicateConfig
		detailView string
		wantErr    string
	}{
		{
			name:       "absent block is accepted",
			dup:        nil,
			detailView: "ticket_detail",
		},
		{
			name:       "known properties are accepted",
			dup:        &DuplicateConfig{Properties: []string{"title", "priority"}},
			detailView: "ticket_detail",
		},
		{
			// AC19c: an entry carrying only `duplicate:` is legitimate. Before
			// TKT-Z8K2FS the empty detail_view was refused outright, so this
			// config could not be written at all.
			name:       "duplicate-only entry loads without detail_view",
			dup:        &DuplicateConfig{Properties: []string{"title"}},
			detailView: "",
		},
		{
			name:       "an entry declaring nothing is still refused",
			dup:        nil,
			detailView: "",
			wantErr:    "declares nothing",
		},
		{
			// AC10: the allowlist is checked against the type, so a typo is a
			// load error rather than a property that silently never carries.
			name:       "unknown property is refused",
			dup:        &DuplicateConfig{Properties: []string{"title", "nope"}},
			detailView: "ticket_detail",
			wantErr:    `unknown property "nope"`,
		},
		{
			// AC10: absence is how a type opts out of narrowing, so an
			// explicitly empty list would be a second spelling of the default.
			name:       "explicitly empty list is refused",
			dup:        &DuplicateConfig{Properties: []string{}},
			detailView: "ticket_detail",
			wantErr:    "duplicate.properties is empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfig(nil, duplicateConfig(tc.dup, tc.detailView), testMetamodel())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected config to load, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

// A scalar `duplicate:` must not be silently coerced into an empty block: the
// block is a mapping, and absence is how a type opts out (AC10).
func TestValidateEntityViews_DuplicateScalarRefused(t *testing.T) {
	var ev EntityViewConfig
	err := yaml.Unmarshal([]byte("detail_view: ticket_detail\nduplicate: true\n"), &ev)
	if err == nil {
		t.Fatalf("expected a scalar duplicate: to fail YAML decoding, got %+v", ev.Duplicate)
	}
}

// CarriesProperty is asked on the absent block far more often than on a present
// one, so its nil contract is pinned rather than left to the call sites.
func TestDuplicateConfig_CarriesProperty(t *testing.T) {
	tests := []struct {
		name string
		dup  *DuplicateConfig
		prop string
		want bool
	}{
		{"nil block carries everything", nil, "title", true},
		{"named property carries", &DuplicateConfig{Properties: []string{"title"}}, "title", true},
		{"unnamed property does not", &DuplicateConfig{Properties: []string{"title"}}, "status", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.dup.CarriesProperty(tc.prop); got != tc.want {
				t.Fatalf("CarriesProperty(%q) = %v, want %v", tc.prop, got, tc.want)
			}
		})
	}
}
