package dataentryconfig

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// iconKeyRe matches an entry in the SPA registry's ICONS map, e.g. `  list: List,`
// or `  "kanban": Kanban,`. Deliberately anchored to two-space indentation so
// it matches map entries and not the surrounding prose or imports.
var iconKeyRe = regexp.MustCompile(`(?m)^\s{2}"?([a-z][a-z0-9-]*)"?:\s*[A-Z]`)

// TestIconAllowlistMatchesFrontend pins the Go icon allowlist to the SPA's
// registry (frontend/src/utils/icons.ts).
//
// The two lists are a contract in BOTH directions, and each kind of drift is a
// distinct bug:
//
//   - A name Go accepts but the SPA cannot render passes config validation and
//     then silently falls back to a generic icon — the author sees a wrong icon
//     with no error anywhere.
//   - A name the SPA knows but Go rejects is a feature an author cannot reach:
//     the config fails to load with "unknown icon" for something that would
//     have rendered perfectly.
//
// Comparing the two real sources (rather than either against a third literal
// list in this file) is what makes this a contract test instead of a
// restatement. The pattern follows TestAppTokensCSSInSyncWithFrontend, which
// pins the color tokens the same way.
func TestIconAllowlistMatchesFrontend(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "frontend", "src", "utils", "icons.ts"))
	if err != nil {
		t.Fatalf("read frontend icons.ts: %v", err)
	}

	// Scope to the ICONS map body so an unrelated object literal elsewhere in
	// the file can't contribute phantom names.
	body := string(src)
	start := strings.Index(body, "export const ICONS")
	if start < 0 {
		t.Fatal("frontend icons.ts no longer exports an ICONS map — update this test with it")
	}
	end := strings.Index(body[start:], "\n}")
	if end < 0 {
		t.Fatal("could not find the end of the ICONS map in frontend icons.ts")
	}
	body = body[start : start+end]

	spa := map[string]bool{}
	for _, m := range iconKeyRe.FindAllStringSubmatch(body, -1) {
		spa[m[1]] = true
	}
	if len(spa) == 0 {
		t.Fatal("parsed zero icon names from frontend icons.ts — the regexp has drifted from the file's shape")
	}

	var missingInGo, missingInSPA []string
	for name := range spa {
		if !ValidIconNames[name] {
			missingInGo = append(missingInGo, name)
		}
	}
	for name := range ValidIconNames {
		if !spa[name] {
			missingInSPA = append(missingInSPA, name)
		}
	}
	sort.Strings(missingInGo)
	sort.Strings(missingInSPA)

	if len(missingInSPA) > 0 {
		t.Errorf("ValidIconNames accepts %v, which the SPA cannot render — "+
			"a config using one would validate and then show a fallback icon with no error",
			missingInSPA)
	}
	if len(missingInGo) > 0 {
		t.Errorf("frontend icons.ts defines %v, which ValidIconNames rejects — "+
			"an author cannot use icons the SPA already supports",
			missingInGo)
	}
}

// TestValidateIconName covers the allowlist check used by kanban columns and
// swimlanes.
func TestValidateIconName(t *testing.T) {
	if errs := validateIconName("", "kanban \"x\": columns[0]"); len(errs) != 0 {
		t.Errorf("empty icon means 'no icon' and must be valid, got %v", errs)
	}
	if errs := validateIconName("inbox", "kanban \"x\": columns[0]"); len(errs) != 0 {
		t.Errorf("known icon must be valid, got %v", errs)
	}
	errs := validateIconName("nope", "kanban \"x\": columns[2]")
	if len(errs) == 0 {
		t.Fatal("unknown icon must be rejected")
	}
	// Locate the offending entry and list the alternatives: a config can hold
	// dozens of columns, and "unknown icon" alone would send the author hunting.
	if !strings.Contains(errs[0], "columns[2]") {
		t.Errorf("message must locate the offending entry, got %q", errs[0])
	}
	if !strings.Contains(errs[0], "valid:") {
		t.Errorf("message must list the valid names, got %q", errs[0])
	}
}

// TestNavigationIconValidation covers `icon:` on navigation entries.
//
// Without an authored icon every list entry gets the same derived glyph, so a
// sidebar of five lists carries no signal beyond its labels. The override has
// to be validated like any other config name: loudly, at load.
func TestNavigationIconValidation(t *testing.T) {
	tests := []struct {
		name    string
		nav     NavigationEntry
		wantErr string
	}{
		{"no icon is fine", NavigationEntry{Label: "Tickets", Dashboard: true}, ""},
		{"known icon on an item", NavigationEntry{Label: "Inbox", Dashboard: true, Icon: "inbox"}, ""},
		{"unknown icon is rejected", NavigationEntry{Label: "Inbox", Dashboard: true, Icon: "nope"}, "unknown icon"},
		{
			"icon on a group is rejected",
			NavigationEntry{Group: "Tickets", Icon: "inbox"},
			"cannot have an icon",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateNavEntry(tc.nav, &Config{})
			if tc.wantErr == "" {
				for _, e := range errs {
					if strings.Contains(e, "icon") {
						t.Errorf("unexpected icon error: %s", e)
					}
				}
				return
			}
			found := false
			for _, e := range errs {
				if strings.Contains(e, tc.wantErr) {
					found = true
				}
			}
			if !found {
				t.Errorf("want an error containing %q, got %v", tc.wantErr, errs)
			}
		})
	}
}
