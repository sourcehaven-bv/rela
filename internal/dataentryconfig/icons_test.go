package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig/icondefs"
)

// The Go allowlist, the SPA registry and the documentation table are all
// GENERATED from internal/dataentryconfig/icondefs, so there is nothing here
// pinning two hand-written lists to each other any more.
//
// The predecessor of this file parsed frontend/src/utils/icons.ts with a regex
// to compare name sets, and needed a count check on top because a partial parse
// (a spread, a nested literal, an aliased import) would silently stop covering
// a name. That whole class of problem is gone — but the reason it existed is
// not: the config allowlist and what the SPA can render must agree, or an
// author gets either a silent fallback glyph or a rejected name that would have
// worked. Generation is now what guarantees it, and TestGenerateIcons_UpToDate
// in cmd/gen-icons is what fails when someone hand-edits an output.

// TestValidIconNames_Generated checks the generated allowlist is present and
// substantial, and that it excludes the reserved no-icon name.
func TestValidIconNames_Generated(t *testing.T) {
	if len(ValidIconNames) < 120 {
		t.Errorf("expected a curated set of at least 120 icons, got %d", len(ValidIconNames))
	}
	if ValidIconNames[NoIcon] {
		t.Errorf("%q is the reserved no-icon name and must not be a renderable icon: "+
			"an author writing it expects nothing to be drawn", NoIcon)
	}
	for name := range ValidIconNames {
		if strings.ToLower(name) != name {
			t.Errorf("icon name %q is not lowercase; names are a public config contract", name)
		}
	}
}

// TestValidIconNames_NoRegression pins every name that existed before the set
// was expanded.
//
// These are load-bearing in a way the other ~200 are not: a project may already
// have authored any of them, so removing or renaming one breaks a config that
// works today. Listed literally rather than derived, because a derived list
// would change alongside the thing it is supposed to be pinning.
func TestValidIconNames_NoRegression(t *testing.T) {
	original := []string{
		"dashboard", "list", "kanban", "search", "calendar", "warning",
		"apps", "settings", "document", "sun", "moon", "inbox", "wrench",
		"done", "clock", "status",
	}
	for _, name := range original {
		if !ValidIconNames[name] {
			t.Errorf("icon %q shipped in an earlier release and must keep working", name)
		}
	}
}

// TestValidateIconName covers the allowlist check used by navigation entries,
// kanban columns and swimlanes.
func TestValidateIconName(t *testing.T) {
	tests := []struct {
		name     string
		icon     string
		wantErr  bool
		contains []string
	}{
		{name: "empty means use the derived icon", icon: ""},
		{name: "a known name", icon: "inbox"},
		{name: "the reserved no-icon name", icon: NoIcon},

		{
			name: "unknown name is rejected with a suggestion",
			icon: "inbx", wantErr: true,
			contains: []string{"columns[2]", `"inbx"`, `did you mean "inbox"`, `"none"`},
		},
		{
			// Names are case-sensitive everywhere else in the config; an
			// exception here would be the only one, and would leave authors
			// guessing which fields are forgiving.
			name: "the no-icon name is case-sensitive",
			icon: "None", wantErr: true,
			contains: []string{`unknown icon "None"`},
		},
		{
			name: "a wild typo suggests nothing rather than something irrelevant",
			icon: "zzzzzzzzzzzz", wantErr: true,
			contains: []string{"unknown icon"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateIconName(tc.icon, `kanban "x": columns[2]`)
			if !tc.wantErr {
				if len(errs) != 0 {
					t.Fatalf("expected %q to be valid, got %v", tc.icon, errs)
				}
				return
			}
			if len(errs) == 0 {
				t.Fatalf("expected %q to be rejected", tc.icon)
			}
			for _, want := range tc.contains {
				if !strings.Contains(errs[0], want) {
					t.Errorf("message must contain %q, got %q", want, errs[0])
				}
			}
		})
	}
}

// TestValidateIconName_MessageStaysShort guards the reason the message stopped
// enumerating every valid name.
//
// At sixteen icons a full list was helpful. At two hundred it is a multi-
// kilobyte wall repeated once per bad entry, which buries the one line the
// author needs. If someone reinstates the enumeration, this fails.
func TestValidateIconName_MessageStaysShort(t *testing.T) {
	errs := validateIconName("nope", "navigation \"x\"")
	if len(errs) != 1 {
		t.Fatalf("want one error, got %v", errs)
	}
	if len(errs[0]) > 300 {
		t.Errorf("error message is %d bytes; it should point at the docs rather than "+
			"list every name:\n%s", len(errs[0]), errs[0])
	}
}

// TestSuggestIcon_IsDeterministic pins tie-breaking.
//
// suggestIcon scans sorted rather than iterating the map, because Go randomizes
// map order: an error message that named a different icon on each run would be
// untestable and would read like a bug to the author seeing it twice.
func TestSuggestIcon_IsDeterministic(t *testing.T) {
	first := suggestIcon("clok")
	for range 50 {
		if got := suggestIcon("clok"); got != first {
			t.Fatalf("suggestion changed between runs: %q then %q", first, got)
		}
	}
	if first != "clock" {
		t.Errorf(`want "clock" for "clok", got %q`, first)
	}
}

// TestNoIconMatchesIcondefs pins the re-export, so the sentinel keeps exactly
// one definition across the two packages that name it.
func TestNoIconMatchesIcondefs(t *testing.T) {
	if NoIcon != icondefs.NoIcon {
		t.Errorf("NoIcon = %q but icondefs.NoIcon = %q", NoIcon, icondefs.NoIcon)
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
		{"none on an item", NavigationEntry{Label: "Plain", Dashboard: true, Icon: NoIcon}, ""},
		{
			// An action entry derives no icon, so `none` asks to suppress
			// something that was never there. Harmless, and rejecting it would
			// make the author reason about which entry kinds have a derived
			// glyph — which is precisely what they should not have to do.
			"none on an action entry is a harmless no-op",
			NavigationEntry{Label: "Run", Action: "sync", Icon: NoIcon},
			"",
		},
		{"unknown icon is rejected", NavigationEntry{Label: "Inbox", Dashboard: true, Icon: "nope"}, "unknown icon"},
		{
			"icon on a group is rejected",
			NavigationEntry{Group: "Tickets", Icon: "inbox"},
			"cannot have an icon",
		},
		{
			// The group-specific message must win: "unknown icon" would send
			// the author looking for a typo in a name that is perfectly valid.
			"none on a group is rejected with the group message",
			NavigationEntry{Group: "Tickets", Icon: NoIcon},
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
