package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestNavEntryIcon covers the authored `icon:` override on a navigation entry.
//
// Without it the icon is derived from the entry's KIND, so every list entry
// gets the same glyph and every board the same one — a sidebar of five lists
// reads as five identical rows distinguished only by their labels. The
// override exists so an author can tell them apart.
func TestNavEntryIcon(t *testing.T) {
	tests := []struct {
		name  string
		entry dataentryconfig.NavigationEntry
		want  string
	}{
		{
			"derived from kind when unset",
			dataentryconfig.NavigationEntry{Label: "Dash", Dashboard: true},
			"dashboard",
		},
		{
			"authored icon overrides the derived one",
			dataentryconfig.NavigationEntry{Label: "Dash", Dashboard: true, Icon: "inbox"},
			"inbox",
		},
		{
			"search keeps its derived icon",
			dataentryconfig.NavigationEntry{Label: "Find", Search: true},
			"search",
		},
		{
			"search can be overridden too",
			dataentryconfig.NavigationEntry{Label: "Find", Search: true, Icon: "clock"},
			"clock",
		},
		{
			// An action derives NO icon, so before the override it was the one
			// entry kind that could never have one. The override is applied
			// after the switch precisely so this case is covered.
			"an action entry can finally have an icon",
			dataentryconfig.NavigationEntry{Label: "Run", Action: "sync", Icon: "progress"},
			"progress",
		},
		{
			"an action without an icon still has none",
			dataentryconfig.NavigationEntry{Label: "Run", Action: "sync"},
			"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := navEntryToSidebarItem(tc.entry)
			if item.Icon != tc.want {
				t.Errorf("Icon = %q, want %q", item.Icon, tc.want)
			}
		})
	}
}
