package dataentryconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TKT-3R7RF3 / RR-Z0GGTO: the widget → accepted-property-types table is
// necessarily encoded twice — once here for config-load validation, once in the
// SPA registry (frontend/src/widgets/registry.ts) as each entry's
// `supportedPropertyTypes`. Two languages, one rule.
//
// The original plan accepted that drift risk on the grounds that ten entries
// don't justify machinery. That reasoning was wrong: a THIRD encoding already
// exists (Metamodel.ResolveWidgetFromType) and it has ALREADY drifted from the
// SPA — it has no `file` case, so it resolves a file property to "text". A
// table that has already drifted once is not a hypothetical risk.
//
// So both sides assert against the shared fixture below. Change the rule and
// this test fails until testdata is updated; the matching Vitest test
// (registry.widgetTable.test.ts) then fails until the SPA agrees. Neither side
// can move alone.
const widgetTableFixture = "testdata/widget_property_types.json"

func TestSectionFieldWidgetTypes_MatchesFixture(t *testing.T) {
	want := loadWidgetFixture(t)

	got := make(map[string][]string, len(sectionFieldWidgetTypes))
	for widget, types := range sectionFieldWidgetTypes {
		sorted := append([]string(nil), types...)
		sort.Strings(sorted)
		got[widget] = sorted
	}

	if len(got) != len(want) {
		t.Fatalf("widget count mismatch: Go has %d (%s), fixture has %d (%s)",
			len(got), strings.Join(sortedWidgetNames(got), ", "),
			len(want), strings.Join(sortedWidgetNames(want), ", "))
	}
	for widget, wantTypes := range want {
		gotTypes, ok := got[widget]
		if !ok {
			t.Errorf("widget %q in fixture but not in sectionFieldWidgetTypes", widget)
			continue
		}
		if strings.Join(gotTypes, ",") != strings.Join(wantTypes, ",") {
			t.Errorf("widget %q accepts %v in Go, %v in fixture", widget, gotTypes, wantTypes)
		}
	}
	for widget := range got {
		if _, ok := want[widget]; !ok {
			t.Errorf("widget %q in sectionFieldWidgetTypes but not in fixture", widget)
		}
	}
}

// The fixture is the contract, so a malformed or missing one must fail loudly
// rather than vacuously pass an empty comparison.
func loadWidgetFixture(t *testing.T) map[string][]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(widgetTableFixture))
	if err != nil {
		t.Fatalf("read widget fixture: %v", err)
	}
	var out map[string][]string
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parse widget fixture: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("widget fixture is empty; it is the cross-language contract")
	}
	for widget, types := range out {
		sort.Strings(types)
		out[widget] = types
	}
	return out
}

func sortedWidgetNames(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
