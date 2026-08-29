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
			// The override is applied after the switch, so it reaches every
			// entry kind — including action, which for a long time was the one
			// kind that could not otherwise have a glyph at all.
			"an authored icon reaches an action entry",
			dataentryconfig.NavigationEntry{Label: "Run", Action: "sync", Icon: "progress"},
			"progress",
		},
		{
			// An action derives a glyph since TKT-EG33Y1. It used to derive
			// none, which was harmless while `icon:` could only override — but
			// `icon: none` left nothing for the collapsed sidebar to draw, and
			// the empty fallback resolved to the generic document glyph.
			"an action derives its own glyph",
			dataentryconfig.NavigationEntry{Label: "Run", Action: "sync"},
			"zap",
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
