package dataentry

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// TestIconNoneEndToEnd walks a real data-entry.yaml fragment from YAML through
// config validation to the sidebar wire payload.
//
// The three unit-level pieces (parse, validate, convert) each pass on their own
// while the feature is broken end to end, because the meaning of `icon: none`
// has to survive all three. This is the one test that would catch a layer
// quietly normalising it away.
func TestIconNoneEndToEnd(t *testing.T) {
	const src = `
navigation:
  - group: "Tickets"
    items:
      - label: "My Tickets"
        list: my_tickets
        icon: inbox
      - label: "Open Tickets"
        list: open_tickets
        icon: none
      - label: "All Tickets"
        list: all_tickets
`
	var cfg dataentryconfig.Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("parse: %v", err)
	}

	// A mixed group must load cleanly: this is the arrangement the feature
	// exists for, so a validation error here would make it unusable.
	if err := dataentryconfig.ValidateConfig([]byte(src), &cfg, nil); err != nil {
		if strings.Contains(err.Error(), "icon") {
			t.Errorf("icon-related validation error: %v", err)
		}
	}

	want := []struct {
		label   string
		icon    string
		derived string
	}{
		{"My Tickets", "inbox", ""},
		{"Open Tickets", dataentryconfig.NoIcon, "list"},
		{"All Tickets", "list", ""},
	}

	items := cfg.Navigation[0].Items
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d", len(items), len(want))
	}
	for i, w := range want {
		got := navEntryToSidebarItem(items[i])
		if got.Icon != w.icon || got.DerivedIcon != w.derived {
			t.Errorf("%s: Icon=%q DerivedIcon=%q, want Icon=%q DerivedIcon=%q",
				w.label, got.Icon, got.DerivedIcon, w.icon, w.derived)
		}

		// The wire shape is what the SPA actually branches on, so assert the
		// JSON rather than only the struct: `omitempty` is precisely the
		// mechanism that would erase a "" sentinel, and a struct-level
		// assertion cannot see it.
		b, err := json.Marshal(got)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		t.Logf("%-14s → %s", w.label, b)

		if w.icon == dataentryconfig.NoIcon {
			var round map[string]any
			if err := json.Unmarshal(b, &round); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if round["icon"] != dataentryconfig.NoIcon {
				t.Errorf("%s: icon did not survive the wire as %q, got %v",
					w.label, dataentryconfig.NoIcon, round["icon"])
			}
			if round["derivedIcon"] != "list" {
				t.Errorf("%s: collapsed-mode fallback lost on the wire, got %v",
					w.label, round["derivedIcon"])
			}
		}
	}
}
