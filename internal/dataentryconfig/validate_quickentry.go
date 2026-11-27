package dataentryconfig

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
)

// quickEntryModifiers and the key rules in quickEntryKey are a subset of the
// names the desktop's Wails accelerator parser accepts, so a shortcut that
// validates here also registers there.
var quickEntryModifiers = map[string]bool{
	"cmdorctrl": true, "cmd": true, "command": true, "ctrl": true,
	"alt": true, "option": true, "optionoralt": true, "shift": true, "super": true,
}

// validateQuickEntry checks the `quick_entry:` block (TKT-24L1H36): the form
// and list exist, the list has an `assign:` over the form's entity type, and
// the shortcut is an accelerator the desktop app can register.
// quickEntrySPAKey opens quick entry in the SPA (useKeyboardShortcuts.ts).
const quickEntrySPAKey = "q"

func validateQuickEntry(cfg *Config) []string {
	q := cfg.QuickEntry
	if q == nil {
		return nil
	}
	var errs []string
	form, ok := cfg.Forms[q.Form]
	switch {
	case q.Form == "":
		errs = append(errs, "quick_entry: form is required")
	case !ok:
		errs = append(errs, fmt.Sprintf("quick_entry: references unknown form %q", q.Form))
	}
	if q.List != "" {
		list, found := cfg.Lists[q.List]
		switch {
		case !found:
			errs = append(errs, fmt.Sprintf("quick_entry: references unknown list %q", q.List))
		case list.Assign == nil:
			errs = append(errs, fmt.Sprintf("quick_entry: list %q has no assign: block to take defaults from", q.List))
		case ok && list.EntityType != form.EntityType:
			errs = append(errs, fmt.Sprintf(
				"quick_entry: list %q is over %q but form %q creates %q",
				q.List, list.EntityType, q.Form, form.EntityType))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(cfg.Actions)) {
		if cfg.Actions[id].Key == quickEntrySPAKey {
			errs = append(errs, fmt.Sprintf(
				"actions: %q declares key %q, which opens quick entry while quick_entry is set; pick another key",
				id, cfg.Actions[id].Key))
		}
	}
	if q.Shortcut != "" {
		if err := checkAccelerator(q.Shortcut); err != "" {
			errs = append(errs, fmt.Sprintf("quick_entry: shortcut %q: %s", q.Shortcut, err))
		}
	}
	return errs
}

// checkAccelerator returns why s is not a usable hotkey, or "" when it is:
// one or more modifiers and one key, joined by "+".
func checkAccelerator(s string) string {
	parts := strings.Split(strings.ToLower(s), "+")
	key := parts[len(parts)-1]
	mods := parts[:len(parts)-1]
	if len(mods) == 0 {
		return "needs at least one modifier, for example CmdOrCtrl+Shift+Space"
	}
	for _, m := range mods {
		if !quickEntryModifiers[m] {
			return fmt.Sprintf("unknown modifier %q", m)
		}
	}
	if !quickEntryKey(key) {
		return fmt.Sprintf("unknown key %q; use a letter, a digit, F1 to F12 or Space", key)
	}
	// why: the shortcut is OS-wide; Cmd+V alone would take it from every app.
	if !strings.HasPrefix(key, "f") || len(key) == 1 {
		if !slices.ContainsFunc(mods, func(m string) bool { return quickEntryShiftAlt[m] }) {
			return "a letter, digit or Space needs Shift or Alt too, so it does not take a key other applications use"
		}
	}
	return ""
}

// quickEntryShiftAlt are the modifiers that keep a shortcut clear of the
// common Cmd/Ctrl editing keys.
var quickEntryShiftAlt = map[string]bool{"shift": true, "alt": true, "option": true, "optionoralt": true}

func quickEntryKey(k string) bool {
	switch {
	case k == "space":
		return true
	case len(k) == 1:
		return (k[0] >= 'a' && k[0] <= 'z') || (k[0] >= '0' && k[0] <= '9')
	case len(k) >= 2 && len(k) <= 3 && k[0] == 'f':
		var n int
		if _, err := fmt.Sscanf(k[1:], "%d", &n); err != nil || strconv.Itoa(n) != k[1:] {
			return false
		}
		return n >= 1 && n <= 12
	}
	return false
}
