package dataentryconfig

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Space is a named entry point onto the one graph: its own navigation, an
// optional home, and the entity types its Create menu offers (TKT-GNKR5H).
//
// A space is not a security boundary. [Space.Permission] hides the space from
// the switcher and protects nothing: the lists and entities behind it stay
// reachable by URL under the normal ACL. See "The configuration is not a
// secret; the data is" in the root CLAUDE.md.
type Space struct {
	// ID names the space in URLs and in API parameters. It must match
	// [spaceIDPattern] and be unique across spaces.
	ID string `yaml:"id" json:"id"`
	// Label is the name the switcher shows. Required.
	Label string `yaml:"label" json:"label"`
	// Icon is an optional glyph name, validated like a navigation entry icon.
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`

	// Permission optionally hides the space from the switcher for principals
	// who do not hold the named global ACL permission. It has the semantics
	// of [NavigationEntry.Permission]: a UX filter for tidiness, not
	// concealment and not access control.
	Permission string `yaml:"permission,omitempty" json:"permission,omitempty"`

	// Home is the page the space opens on: one navigation destination, never
	// a group, an action or an entry with `status:` or `open:`. When absent,
	// the space opens on the first destination in its navigation that the
	// principal can see.
	//
	// Nil: accepted, means "no explicit home".
	Home *NavigationEntry `yaml:"home,omitempty" json:"home,omitempty"`

	// Create lists the entity types the space's Create menu offers, in order.
	Create []string `yaml:"create,omitempty" json:"create,omitempty"`

	// Navigation is the space's sidebar, with the same shape and validation
	// as the top-level `navigation:`.
	Navigation []NavigationEntry `yaml:"navigation,omitempty" json:"navigation,omitempty"`
}

// spaceIDPattern is the shape of [Space.ID]: short, lowercase and safe as a
// URL path segment and as a nav status key prefix (it cannot contain ':').
var spaceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

// NavTree is one navigation tree of a config: the top-level `navigation:`
// (Space "") or the navigation of one space.
type NavTree struct {
	Space   string
	Entries []NavigationEntry
}

// NavigationTrees returns every navigation tree in the config: the top-level
// navigation first, then each space's in config order. Code that walks the
// navigation walks every tree through this, so a space's entries are never
// skipped by a check the top-level entries get.
func NavigationTrees(cfg *Config) []NavTree {
	if cfg == nil {
		return nil
	}
	trees := make([]NavTree, 0, 1+len(cfg.Spaces))
	trees = append(trees, NavTree{Entries: cfg.Navigation})
	for _, sp := range cfg.Spaces {
		trees = append(trees, NavTree{Space: sp.ID, Entries: sp.Navigation})
	}
	return trees
}

// HasSpaces reports whether the config declares `spaces:`. Without it the
// top-level `navigation:` is the only navigation.
func (c *Config) HasSpaces() bool {
	return c != nil && len(c.Spaces) > 0
}

// validateSpaces checks `spaces:`. The navigation inside each space is
// checked by validateNavigation, which walks every [NavigationTrees] tree.
func validateSpaces(cfg *Config, meta *metamodel.Metamodel) []string {
	if !cfg.HasSpaces() {
		return nil
	}
	var errs []string
	if len(cfg.Navigation) > 0 {
		errs = append(errs, "navigation and spaces are both set: move the top-level navigation into a space "+
			"(with spaces:, each space has its own navigation:)")
	}

	seen := make(map[string]bool, len(cfg.Spaces))
	for i, sp := range cfg.Spaces {
		name := fmt.Sprintf("spaces[%s]", sp.ID)
		switch {
		case sp.ID == "":
			errs = append(errs, fmt.Sprintf("spaces[%d]: id is required", i))
			name = fmt.Sprintf("spaces[%d]", i)
		case !spaceIDPattern.MatchString(sp.ID):
			errs = append(errs, fmt.Sprintf("spaces[%d]: invalid id %q (must match %s)", i, sp.ID, spaceIDPattern))
			name = fmt.Sprintf("spaces[%d]", i)
		case seen[sp.ID]:
			errs = append(errs, name+": duplicate id")
		}
		seen[sp.ID] = true

		if strings.TrimSpace(sp.Label) == "" {
			errs = append(errs, name+": label is required")
		}
		errs = append(errs, validateIconName(sp.Icon, name)...)
		errs = append(errs, validateSpaceCreate(sp, name, meta)...)
		if sp.Home != nil {
			errs = append(errs, validateSpaceHome(*sp.Home, name+".home", cfg)...)
		}
	}
	return errs
}

// validateSpaceCreate checks that each `create:` type exists in the metamodel.
func validateSpaceCreate(sp Space, name string, meta *metamodel.Metamodel) []string {
	var errs []string
	listed := make(map[string]bool, len(sp.Create))
	for _, typ := range sp.Create {
		if meta != nil {
			if _, ok := meta.GetEntityDef(typ); !ok {
				errs = append(errs, fmt.Sprintf("%s.create: unknown entity type %q", name, typ))
			}
		}
		if listed[typ] {
			errs = append(errs, fmt.Sprintf("%s.create: entity type %q is listed twice", name, typ))
		}
		listed[typ] = true
	}
	return errs
}

// validateSpaceHome checks a space's `home:`. It must name exactly the kind of
// target a sidebar item links to, so the SPA can derive its URL the same way.
func validateSpaceHome(home NavigationEntry, name string, cfg *Config) []string {
	if home.IsGroup() {
		return []string{name + ": must be a destination, not a group"}
	}
	var errs []string
	if home.Action != "" {
		errs = append(errs, name+": an action is not a destination (use a list, kanban, calendar, gantt, "+
			"document, page, dashboard, search or settings)")
	} else if !home.isDestination() {
		errs = append(errs, name+": names no destination (set one of list, kanban, calendar, gantt, "+
			"document, page, dashboard, search or settings)")
	}
	if len(home.Status) > 0 {
		errs = append(errs, name+": status is not supported (a home is not a sidebar row)")
	}
	if home.ItemsFrom != nil {
		errs = append(errs, name+": items_from is not supported (it fills a navigation group)")
	}
	if home.Open != "" {
		errs = append(errs, name+": open is not supported (a home always opens as a page)")
	}
	if home.Permission != "" {
		errs = append(errs, name+": permission is not supported (set permission on the space instead)")
	}
	for _, e := range navTargetErrors(home, cfg) {
		errs = append(errs, name+": "+e)
	}
	return errs
}

// isDestination reports whether the entry links to a page.
func (n NavigationEntry) isDestination() bool {
	return n.List != "" || n.Kanban != "" || n.Calendar != "" || n.Gantt != "" ||
		n.Document != "" || n.Page != "" || n.Dashboard || n.Search || n.Settings
}
