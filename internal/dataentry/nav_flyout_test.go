package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestNavEntryFlyout pins that `open: flyout` reaches the sidebar payload as
// the list to open, and that the entry keeps its href: a modified click and
// the panel's expand control still need the full page.
func TestNavEntryFlyout(t *testing.T) {
	tests := []struct {
		name     string
		entry    dataentryconfig.NavigationEntry
		wantList string
	}{
		{"page by default", dataentryconfig.NavigationEntry{Label: "All", List: "all"}, ""},
		{"explicit page", dataentryconfig.NavigationEntry{Label: "All", List: "all", Open: "page"}, ""},
		{"flyout", dataentryconfig.NavigationEntry{Label: "Mine", List: "mine", Open: "flyout"}, "mine"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := navEntryToSidebarItem(tc.entry, nil)
			if got.Href != "/list/"+tc.entry.List {
				t.Errorf("Href = %q, want /list/%s", got.Href, tc.entry.List)
			}
			switch {
			case tc.wantList == "" && got.Flyout != nil:
				t.Errorf("Flyout = %+v, want nil", got.Flyout)
			case tc.wantList != "" && (got.Flyout == nil || got.Flyout.List != tc.wantList):
				t.Errorf("Flyout = %+v, want list %q", got.Flyout, tc.wantList)
			}
		})
	}
}
