package dataentryconfig

import (
	"strings"
	"testing"
)

func TestValidateQuickEntry(t *testing.T) {
	base := func(q *QuickEntry) *Config {
		return &Config{
			Forms: map[string]Form{"new_todo": {EntityType: "todo"}, "new_note": {EntityType: "note"}},
			Lists: map[string]List{
				"inbox": {EntityType: "todo", Assign: &Assign{}},
				"all":   {EntityType: "todo"},
			},
			QuickEntry: q,
		}
	}
	cases := []struct {
		name string
		q    *QuickEntry
		want string // substring of the one error; "" for none
	}{
		{"absent", nil, ""},
		{"form only", &QuickEntry{Form: "new_todo"}, ""},
		{"form and list", &QuickEntry{Form: "new_todo", List: "inbox"}, ""},
		{"custom shortcut", &QuickEntry{Form: "new_todo", Shortcut: "Cmd+Option+N"}, ""},
		{"function key", &QuickEntry{Form: "new_todo", Shortcut: "Ctrl+F12"}, ""},
		{"no form", &QuickEntry{}, "form is required"},
		{"unknown form", &QuickEntry{Form: "nope"}, `unknown form "nope"`},
		{"unknown list", &QuickEntry{Form: "new_todo", List: "nope"}, `unknown list "nope"`},
		{"list without assign", &QuickEntry{Form: "new_todo", List: "all"}, "no assign: block"},
		{"type mismatch", &QuickEntry{Form: "new_note", List: "inbox"}, `is over "todo"`},
		{"no modifier", &QuickEntry{Form: "new_todo", Shortcut: "Q"}, "at least one modifier"},
		{"unknown key", &QuickEntry{Form: "new_todo", Shortcut: "Cmd+Banana"}, `unknown key "banana"`},
		{"unknown modifier", &QuickEntry{Form: "new_todo", Shortcut: "Hyper+K"}, `unknown modifier "hyper"`},
		{"F13", &QuickEntry{Form: "new_todo", Shortcut: "Cmd+F13"}, "unknown key"},
		{"Cmd+V", &QuickEntry{Form: "new_todo", Shortcut: "Cmd+V"}, "needs Shift or Alt"},
		{"Ctrl+Space", &QuickEntry{Form: "new_todo", Shortcut: "Ctrl+Space"}, "needs Shift or Alt"},
		{"F01", &QuickEntry{Form: "new_todo", Shortcut: "Cmd+F01"}, "unknown key"},
		{"empty key", &QuickEntry{Form: "new_todo", Shortcut: "Cmd+"}, "unknown key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateQuickEntry(base(tc.q))
			if tc.want == "" {
				if len(errs) != 0 {
					t.Fatalf("want no errors, got %v", errs)
				}
				return
			}
			if len(errs) != 1 || !strings.Contains(errs[0], tc.want) {
				t.Fatalf("want one error containing %q, got %v", tc.want, errs)
			}
		})
	}
}

func TestValidateQuickEntry_ActionKeyQ(t *testing.T) {
	cfg := &Config{
		Forms:   map[string]Form{"new_todo": {EntityType: "todo"}},
		Actions: map[string]Action{"quit": {Key: "q"}, "shifted": {Key: "Q"}, "other": {Key: "x"}},
	}
	if errs := validateQuickEntry(cfg); len(errs) != 0 {
		t.Fatalf("without quick_entry, key q is free: %v", errs)
	}
	cfg.QuickEntry = &QuickEntry{Form: "new_todo"}
	errs := validateQuickEntry(cfg)
	if len(errs) != 1 || !strings.Contains(errs[0], `"quit" declares key "q"`) {
		t.Fatalf("got %v", errs)
	}
}

func TestQuickEntryAccelerator(t *testing.T) {
	if got := (&QuickEntry{}).Accelerator(); got != DefaultQuickEntryShortcut {
		t.Errorf("default: got %q", got)
	}
	if got := (&QuickEntry{Shortcut: "Cmd+K"}).Accelerator(); got != "Cmd+K" {
		t.Errorf("custom: got %q", got)
	}
	if msg := checkAccelerator(DefaultQuickEntryShortcut); msg != "" {
		t.Errorf("the default must validate: %s", msg)
	}
}
