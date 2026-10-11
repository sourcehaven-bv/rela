package dataentryconfig

import (
	"strings"
	"testing"
)

// TestValidateConfig_MentionsAndByType sweeps the load checks for the
// `mentioned_by` traverse step and the per-type section lists (TKT-HF0D5W).
func TestValidateConfig_MentionsAndByType(t *testing.T) {
	mentions := []ViewTraverse{{From: "entry", MentionedBy: true, CollectAs: "mentions"}}
	withStep := func(step ViewTraverse) []ViewTraverse { return []ViewTraverse{step} }
	list := ViewSection{Source: "mentions", Display: "list"}

	tests := []struct {
		name     string
		traverse []ViewTraverse
		sections []ViewSection
		wantErr  string // substring; empty means the config must validate
	}{
		{
			name:     "valid table with per-type columns and a fallback",
			traverse: mentions,
			sections: []ViewSection{{
				Source: "mentions", Display: "table",
				Columns:       []ListColumn{{Property: "name"}},
				ColumnsByType: map[string][]ListColumn{"ticket": {{Property: "title"}, {Property: "status"}}},
				Sort:          []SortSpec{{Property: "status"}},
			}},
		},
		{
			name:     "valid list with per-type fields",
			traverse: mentions,
			sections: []ViewSection{{
				Source: "mentions", Display: "list",
				FieldsByType: map[string][]ViewSectionField{"ticket": {{Property: "status"}}},
			}},
		},
		{
			name: "valid where",
			traverse: withStep(ViewTraverse{From: "entry", MentionedBy: true, CollectAs: "mentions",
				Where: "status = open"}),
			sections: []ViewSection{list},
		},
		{
			name:     "from another collection",
			traverse: []ViewTraverse{{From: "entry", Follow: "blocks", CollectAs: "blocked"}, {From: "blocked", MentionedBy: true, CollectAs: "mentions"}},
			sections: []ViewSection{list, {Source: "blocked", Display: "list"}},
			wantErr:  "takes from: entry only",
		},
		{
			name:     "with follow",
			traverse: withStep(ViewTraverse{From: "entry", MentionedBy: true, Follow: "blocks", CollectAs: "mentions"}),
			sections: []ViewSection{list},
			wantErr:  "cannot be combined with follow",
		},
		{
			name:     "with max_depth",
			traverse: withStep(ViewTraverse{From: "entry", MentionedBy: true, MaxDepth: 2, CollectAs: "mentions"}),
			sections: []ViewSection{list},
			wantErr:  "cannot be combined with max_depth",
		},
		{
			name:     "shared collect_as",
			traverse: []ViewTraverse{mentions[0], {From: "entry", Follow: "blocks", CollectAs: "mentions"}},
			sections: []ViewSection{list},
			wantErr:  "needs its own collection",
		},
		{
			name:     "bad where",
			traverse: withStep(ViewTraverse{From: "entry", MentionedBy: true, CollectAs: "mentions", Where: "=="}),
			sections: []ViewSection{list},
			wantErr:  "invalid where",
		},
		{
			name:     "collected but never shown",
			traverse: mentions,
			sections: []ViewSection{{Source: "entry", Display: "content"}},
			wantErr:  "no section shows them",
		},
		{
			name: "only another step reads it",
			traverse: []ViewTraverse{mentions[0],
				{From: "mentions", Follow: "belongs-to", CollectAs: "cats"}},
			sections: []ViewSection{{Source: "cats", Display: "list"}},
			wantErr:  "no section shows them",
		},
		{
			name:     "nested under nothing",
			traverse: mentions,
			sections: []ViewSection{{Source: "entry", Display: DisplayNested, Children: "mentions"}},
			wantErr:  "backlinks carry no parent",
		},
		{
			name:     "fields_by_type on a table",
			traverse: mentions,
			sections: []ViewSection{{Source: "mentions", Display: "table", Columns: []ListColumn{{Property: "title"}},
				FieldsByType: map[string][]ViewSectionField{"ticket": {{Property: "status"}}}}},
			wantErr: "sets fields_by_type, which display \"table\" does not take",
		},
		{
			name:     "columns_by_type on a list",
			traverse: mentions,
			sections: []ViewSection{{Source: "mentions", Display: "list",
				ColumnsByType: map[string][]ListColumn{"ticket": {{Property: "status"}}}}},
			wantErr: "sets columns_by_type, which display \"list\" does not take",
		},
		{
			name:     "columns_by_type with group_by",
			traverse: mentions,
			sections: []ViewSection{{Source: "mentions", Display: "table", GroupBy: "status",
				Columns:       []ListColumn{{Property: "title"}},
				ColumnsByType: map[string][]ListColumn{"ticket": {{Property: "status"}}}}},
			wantErr: "already groups the table by type",
		},
		{
			name:     "per-type list on the entry",
			traverse: mentions,
			sections: []ViewSection{list, {Source: "entry", Display: "properties",
				FieldsByType: map[string][]ViewSectionField{"ticket": {{Property: "status"}}}}},
			wantErr: "per-type list on source: entry",
		},
		{
			name:     "unknown type key",
			traverse: mentions,
			sections: []ViewSection{{Source: "mentions", Display: "list",
				FieldsByType: map[string][]ViewSectionField{"gadget": {{Property: "status"}}}}},
			wantErr: `unknown entity type "gadget"`,
		},
		{
			name:     "property the keyed type lacks",
			traverse: mentions,
			sections: []ViewSection{{Source: "mentions", Display: "table", Columns: []ListColumn{{Property: "title"}},
				ColumnsByType: map[string][]ListColumn{"ticket": {{Property: "name"}}}}},
			wantErr: `property "name" not in entity "ticket"`,
		},
		{
			name:     "empty per-type list",
			traverse: mentions,
			sections: []ViewSection{{Source: "mentions", Display: "list",
				FieldsByType: map[string][]ViewSectionField{"ticket": {}}}},
			wantErr: "is empty",
		},
		{
			name:     "no fallback columns",
			traverse: mentions,
			sections: []ViewSection{{Source: "mentions", Display: "table",
				ColumnsByType: map[string][]ListColumn{"ticket": {{Property: "status"}}}}},
			wantErr: "needs a columns: fallback",
		},
		{
			name:     "fallback field no remaining type declares",
			traverse: mentions,
			sections: []ViewSection{{Source: "mentions", Display: "list",
				Fields:       []ViewSectionField{{Property: "priority"}},
				FieldsByType: map[string][]ViewSectionField{"ticket": {{Property: "status"}}}}},
			wantErr: "declared by none of the types it applies to",
		},
		{
			name:     "key a relation never reaches",
			traverse: []ViewTraverse{{From: "entry", Follow: "belongs-to", CollectAs: "cats"}},
			sections: []ViewSection{{Source: "cats", Display: "list",
				FieldsByType: map[string][]ViewSectionField{"ticket": {{Property: "status"}}}}},
			wantErr: "which this collection never holds",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{Views: map[string]ViewConfig{
				"test": {Entry: ViewEntry{Type: "ticket"}, Traverse: tc.traverse, Sections: tc.sections},
			}}
			err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected valid config, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateConfig_MentionsRefusedInSidePanel(t *testing.T) {
	cfg := &Config{Forms: map[string]Form{
		"ticket_form": {
			EntityType: "ticket",
			Fields:     []FormField{{Property: "title"}},
			SidePanel: &SidePanelConfig{
				Traverse: []ViewTraverse{{From: "entry", MentionedBy: true, CollectAs: "mentions"}},
				Sections: []ViewSection{{Heading: "M", Source: "mentions", Display: "list",
					FieldsByType: map[string][]ViewSectionField{"ticket": {{Property: "status"}}}}},
			},
		},
	}}
	err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel())
	if err == nil {
		t.Fatal("expected mentioned_by in a side panel to be refused")
	}
	for _, want := range []string{"side_panel traverse[0] sets mentioned_by", "side_panel section[0] sets a per-type list"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

// An entity_views entry that only switches the panel configures something,
// so it is not refused as empty.
func TestValidateConfig_MentionedByPanelOnlyEntry(t *testing.T) {
	off := false
	cfg := &Config{EntityViews: map[string]EntityViewConfig{"ticket": {MentionedByPanel: &off}}}
	if err := ValidateConfig([]byte(`version: "1.0"`), cfg, testMetamodel()); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}
