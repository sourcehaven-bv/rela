package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestNavEntryPile pins that a `pile:` entry reaches the sidebar as the pile
// name with no href, takes the pile glyph, and is labeled by the name when
// the config gives no label.
func TestNavEntryPile(t *testing.T) {
	tests := []struct {
		name      string
		entry     dataentryconfig.NavigationEntry
		wantLabel string
		wantIcon  string
	}{
		{"name as label", dataentryconfig.NavigationEntry{Pile: "Notifications"}, "Notifications", "layers"},
		{"own label and icon", dataentryconfig.NavigationEntry{Label: "Inbox", Pile: "Notifications", Icon: "bell"},
			"Inbox", "bell"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := navEntryToSidebarItem(tc.entry, nil)
			if got.Pile != "Notifications" || got.Href != "" {
				t.Errorf("Pile, Href = %q, %q; want Notifications, empty", got.Pile, got.Href)
			}
			if got.Label != tc.wantLabel || got.Icon != tc.wantIcon {
				t.Errorf("Label, Icon = %q, %q; want %q, %q", got.Label, got.Icon, tc.wantLabel, tc.wantIcon)
			}
		})
	}
}
