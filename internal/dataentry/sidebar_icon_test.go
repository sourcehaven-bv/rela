package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestNavEntryToSidebarItem_Icon covers the three-way icon resolution on the
// wire.
//
// Three distinct author intents have to survive the trip to the client:
//
//	no icon: key      → use the glyph derived from this entry's kind
//	icon: <name>      → use that glyph
//	icon: none        → draw nothing, and reserve the column
//
// The third is the one worth testing carefully. It travels as the literal
// "none" rather than as "", because the field is `json:"icon,omitempty"` — an
// empty string is dropped from the payload and becomes indistinguishable from
// an entry that never had an icon at all. Asserting Icon == "" here would
// therefore pass without proving anything.
func TestNavEntryToSidebarItem_Icon(t *testing.T) {
	// Every entry kind, so a future kind added to the switch without an icon
	// branch shows up as a failure rather than as a blank sidebar row.
	kinds := []struct {
		name    string
		entry   dataentryconfig.NavigationEntry
		derived string
	}{
		{"list", dataentryconfig.NavigationEntry{Label: "L", List: "l"}, "list"},
		{"kanban", dataentryconfig.NavigationEntry{Label: "K", Kanban: "k"}, "kanban"},
		{"calendar", dataentryconfig.NavigationEntry{Label: "C", Calendar: "c"}, "calendar"},
		{"dashboard", dataentryconfig.NavigationEntry{Label: "D", Dashboard: true}, "dashboard"},
		{"search", dataentryconfig.NavigationEntry{Label: "S", Search: true}, "search"},
		{"settings", dataentryconfig.NavigationEntry{Label: "G", Settings: true}, "settings"},
		{"document", dataentryconfig.NavigationEntry{Label: "Doc", Document: "d"}, "document"},
		// An action derives a glyph too, since TKT-EG33Y1. It used to derive
		// none — harmless while icon: could only override, but `icon: none`
		// then had nothing to fall back to in the collapsed sidebar and
		// resolved to the generic document glyph: a button that fires a
		// mutation, drawn identically to a link to a document.
		{"action", dataentryconfig.NavigationEntry{Label: "A", Action: "a"}, "zap"},
	}

	for _, k := range kinds {
		t.Run(k.name+"/derives its kind glyph", func(t *testing.T) {
			got := navEntryToSidebarItem(k.entry)
			if got.Icon != k.derived {
				t.Errorf("Icon = %q, want the kind-derived %q", got.Icon, k.derived)
			}
			if got.DerivedIcon != "" {
				t.Errorf("DerivedIcon = %q; it should be sent only when the icon "+
					"is suppressed, not duplicated onto every entry", got.DerivedIcon)
			}
		})

		t.Run(k.name+"/an authored name wins", func(t *testing.T) {
			e := k.entry
			e.Icon = "inbox"
			if got := navEntryToSidebarItem(e); got.Icon != "inbox" {
				t.Errorf("Icon = %q, want the authored \"inbox\"", got.Icon)
			}
		})

		t.Run(k.name+"/none suppresses the derived glyph", func(t *testing.T) {
			e := k.entry
			e.Icon = dataentryconfig.NoIcon
			got := navEntryToSidebarItem(e)

			if got.Icon != dataentryconfig.NoIcon {
				t.Errorf("Icon = %q, want %q — an empty string would be dropped by "+
					"omitempty and read as 'no icon was ever chosen'",
					got.Icon, dataentryconfig.NoIcon)
			}
			// The collapsed sidebar hides labels, so it needs something to draw;
			// it cannot reconstruct the derived glyph itself, because the
			// suppression already happened by the time it sees the payload.
			if got.DerivedIcon != k.derived {
				t.Errorf("DerivedIcon = %q, want %q for the collapsed fallback",
					got.DerivedIcon, k.derived)
			}
		})
	}
}

// TestNavEntryToSidebarItem_EmptyIconUnchanged pins the meaning `none` exists
// to avoid overloading.
//
// An empty icon: has always meant "use the derived glyph". If it ever came to
// mean "draw nothing", every config written before this feature would quietly
// lose its icons.
func TestNavEntryToSidebarItem_EmptyIconUnchanged(t *testing.T) {
	got := navEntryToSidebarItem(dataentryconfig.NavigationEntry{
		Label: "All Tickets", List: "all", Icon: "",
	})
	if got.Icon != "list" {
		t.Errorf("Icon = %q, want \"list\": an empty icon: means 'derive one', "+
			"not 'draw nothing'", got.Icon)
	}
}

// TestDerivedIconsAreValidNames pins the OTHER half of the icon-name coupling.
//
// The SPA's direct references are guarded by generated named exports, so a
// rename breaks the TypeScript build. The server's are not: it emits a glyph
// name as a plain string on the wire, and the SPA resolves it through an
// allowlist. If the canonical table renamed an entry and this handler kept the
// old name, resolveIcon would miss and EVERY navigation entry of that kind
// would quietly render the fallback glyph — no build error, no test failure,
// and an assertion against the same stale literal would agree with the bug.
//
// Asserting against the allowlist rather than a literal is what makes this
// catch it.
func TestDerivedIconsAreValidNames(t *testing.T) {
	kinds := []dataentryconfig.NavigationEntry{
		{Label: "L", List: "l"},
		{Label: "K", Kanban: "k"},
		{Label: "C", Calendar: "c"},
		{Label: "D", Dashboard: true},
		{Label: "S", Search: true},
		{Label: "G", Settings: true},
		{Label: "Doc", Document: "d"},
		{Label: "A", Action: "a"},
	}
	for _, entry := range kinds {
		got := navEntryToSidebarItem(entry)
		if got.Icon == "" {
			t.Errorf("%s derives no glyph; `icon: none` on it would have nothing to "+
				"fall back on when the sidebar collapses", entry.Label)
			continue
		}
		if !dataentryconfig.ValidIconNames[got.Icon] {
			t.Errorf("%s derives %q, which is not in the allowlist — the SPA would "+
				"render the fallback glyph for every entry of this kind",
				entry.Label, got.Icon)
		}
	}
}

// TestActionEntryNoneHasACollapsedFallback covers the case that made an action
// button indistinguishable from a document link.
func TestActionEntryNoneHasACollapsedFallback(t *testing.T) {
	got := navEntryToSidebarItem(dataentryconfig.NavigationEntry{
		Label: "Archive Sprint", Action: "archive_sprint", Icon: dataentryconfig.NoIcon,
	})

	if got.DerivedIcon == "" {
		t.Fatal("an action entry with icon: none sends no collapsed fallback, so the " +
			"client resolves nothing and lands on the generic document glyph — a " +
			"button that runs a mutation, drawn like a link to a document")
	}
	if got.DerivedIcon == "document" {
		t.Errorf("the collapsed fallback is %q, which is what a real document entry "+
			"shows; the two must not be indistinguishable", got.DerivedIcon)
	}
	if !dataentryconfig.ValidIconNames[got.DerivedIcon] {
		t.Errorf("collapsed fallback %q is not a renderable icon", got.DerivedIcon)
	}
}
