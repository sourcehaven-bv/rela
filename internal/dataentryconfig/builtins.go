package dataentryconfig

import (
	"fmt"
	"slices"
	"strings"
)

// BuiltinRow names one of the rows the sidebar pins above the configured
// navigation, the as-of date picker in the sidebar header, or the Settings,
// About and Shortcuts buttons in the sidebar footer. The names are the keys
// of a `builtins:` block.
type BuiltinRow string

// The built-in sidebar rows, in the order the sidebar shows them.
const (
	BuiltinSearch    BuiltinRow = "search"
	BuiltinInbox     BuiltinRow = "inbox"
	BuiltinFlowTasks BuiltinRow = "flow_tasks"
	BuiltinFlowRuns  BuiltinRow = "flow_runs"
	BuiltinAnalysis  BuiltinRow = "analysis"
	// BuiltinAsOf is the date picker in the sidebar header, not a row. It
	// shows only where the storage keeps version history.
	BuiltinAsOf BuiltinRow = "as_of"
	// BuiltinShortcuts is the Shortcuts button in the sidebar footer, which
	// opens the keyboard shortcuts help. Not a row either.
	BuiltinShortcuts BuiltinRow = "shortcuts"
	// BuiltinSettings is the Settings button in the sidebar footer. Hiding
	// it tidies the footer for users with nothing to set there; the
	// settings page stays reachable by URL, and its saves keep checking
	// settings:edit.
	BuiltinSettings BuiltinRow = "settings"
	// BuiltinAbout is the About button in the sidebar footer. It shows only
	// when the About dialog has something to say; this can only hide it
	// further.
	BuiltinAbout BuiltinRow = "about"
)

// BuiltinRows lists every built-in row in sidebar order, then the as-of
// picker and the footer buttons.
var BuiltinRows = []BuiltinRow{
	BuiltinSearch, BuiltinInbox, BuiltinFlowTasks, BuiltinFlowRuns, BuiltinAnalysis, BuiltinAsOf,
	BuiltinShortcuts, BuiltinSettings, BuiltinAbout,
}

// BuiltinConfig says whether one built-in sidebar row is shown.
//
// Without an entry a row is shown as before; the inbox and flow rows still
// appear only when the project has channels or flows. Set Hidden alone, or
// Permission, HiddenOnPhone or both.
//
// Every field is a UX filter, the same as [NavigationEntry.Permission]: they
// keep a row out of the sidebar and protect nothing. The page behind the row
// stays reachable by URL and applies the normal ACL to what it shows. The
// configuration is not a secret, so `/api/v1/_config` serves this block to
// everyone (see "The configuration is not a secret; the data is" in the root
// CLAUDE.md).
type BuiltinConfig struct {
	// Hidden removes the row for every principal.
	Hidden bool `yaml:"hidden,omitempty" json:"hidden,omitempty"`
	// Permission shows the row only to principals who hold the named global
	// ACL permission.
	Permission string `yaml:"permission,omitempty" json:"permission,omitempty"`
	// HiddenOnPhone removes the row on a phone-sized screen for every
	// principal and keeps it on wider screens. The SPA applies it, since only
	// the browser knows the screen width.
	HiddenOnPhone bool `yaml:"hidden_on_phone,omitempty" json:"hidden_on_phone,omitempty"`
}

// Builtins configures the built-in sidebar rows, keyed by row name. Nil:
// accepted, means every row is shown.
type Builtins map[BuiltinRow]BuiltinConfig

// Hidden returns the rows to leave out of the sidebar, in sidebar order.
// permits answers whether the principal holds a permission; the caller
// supplies the same check it uses for a navigation entry's `permission:`.
func (b Builtins) Hidden(permits func(permission string) bool) []BuiltinRow {
	var out []BuiltinRow
	for _, row := range BuiltinRows {
		c, ok := b[row]
		if !ok {
			continue
		}
		if c.Hidden || (c.Permission != "" && !permits(c.Permission)) {
			out = append(out, row)
		}
	}
	return out
}

// HiddenOnPhone returns the rows to leave out on a phone-sized screen, in
// sidebar order. A row Hidden for everyone is not listed: Hidden returns it.
func (b Builtins) HiddenOnPhone() []BuiltinRow {
	var out []BuiltinRow
	for _, row := range BuiltinRows {
		if c, ok := b[row]; ok && c.HiddenOnPhone && !c.Hidden {
			out = append(out, row)
		}
	}
	return out
}

// validateBuiltins checks one `builtins:` block. name prefixes the messages:
// "builtins" for the top level, "spaces[<id>].builtins" for a space.
func validateBuiltins(b Builtins, name string) []string {
	known := make(map[BuiltinRow]bool, len(BuiltinRows))
	valid := make([]string, 0, len(BuiltinRows))
	for _, row := range BuiltinRows {
		known[row] = true
		valid = append(valid, string(row))
	}
	rows := make([]string, 0, len(b))
	for row := range b {
		rows = append(rows, string(row))
	}
	slices.Sort(rows)

	var errs []string
	for _, key := range rows {
		row := BuiltinRow(key)
		if !known[row] {
			errs = append(errs, fmt.Sprintf("%s: unknown built-in row %q (valid rows: %s)",
				name, key, strings.Join(valid, ", ")))
			continue
		}
		c := b[row]
		switch {
		case c.Hidden && strings.TrimSpace(c.Permission) != "":
			errs = append(errs, fmt.Sprintf("%s.%s: set hidden or permission, not both "+
				"(a hidden row is hidden from every principal)", name, key))
		case c.Hidden && c.HiddenOnPhone:
			errs = append(errs, fmt.Sprintf("%s.%s: set hidden or hidden_on_phone, not both "+
				"(a hidden row is hidden on every screen)", name, key))
		case !c.Hidden && !c.HiddenOnPhone && strings.TrimSpace(c.Permission) == "":
			errs = append(errs, fmt.Sprintf("%s.%s: set hidden: true, hidden_on_phone: true or a permission "+
				"(without any of them the row is shown, so leave the entry out)", name, key))
		case c.Permission != strings.TrimSpace(c.Permission):
			errs = append(errs, fmt.Sprintf("%s.%s: permission %q has leading or trailing spaces",
				name, key, c.Permission))
		}
	}
	return errs
}

// validateAllBuiltins checks the top-level `builtins:` and each space's. With
// `spaces:` the top-level block is refused, like the top-level navigation:
// each space has its own sidebar, so its rows are configured on the space.
func validateAllBuiltins(cfg *Config) []string {
	errs := validateBuiltins(cfg.Builtins, "builtins")
	if cfg.HasSpaces() && len(cfg.Builtins) > 0 {
		errs = append(errs, "builtins and spaces are both set: move the top-level builtins into each space "+
			"(with spaces:, each space has its own builtins:)")
	}
	for i, sp := range cfg.Spaces {
		name := fmt.Sprintf("spaces[%s].builtins", sp.ID)
		if sp.ID == "" {
			name = fmt.Sprintf("spaces[%d].builtins", i)
		}
		errs = append(errs, validateBuiltins(sp.Builtins, name)...)
	}
	return errs
}
