package dataentryconfig

import (
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Page is a named screen that shows several views of one subject as tabs, such
// as a board, a table and a timeline of the same tickets (TKT-ITQ0HL).
//
// The tabs belong to the page, not to the navigation entry that links to it.
// A bookmarked URL names the page and has no navigation entry to take tabs
// from, and one view can sit under several entries, so the tab bar is the
// same however the user arrived.
//
// A page with EntityType is an entity page: it shows one entity of that type,
// the anchor, named in the URL as `/p/<page>/<entity>/<tab>`. Each tab shows
// its view narrowed to what the anchor is related to (see [PageTabScope]).
type Page struct {
	// Label is the page title in the header, and the sidebar label of a
	// `page:` navigation entry that sets none. Required. On an entity page
	// the header shows the anchor's title, and Label names the kind of page.
	Label string `yaml:"label" json:"label"`
	// Icon is an optional glyph name, validated like a navigation entry icon.
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`
	// EntityType makes this an entity page for entities of that type.
	EntityType string `yaml:"entity_type,omitempty" json:"entity_type,omitempty"`
	// Badge names a property of EntityType whose value the header shows as
	// a pill beside the anchor's title. Only on an entity page.
	Badge string `yaml:"badge,omitempty" json:"badge,omitempty"`
	// Tabs are the views of the page, in tab-bar order. At least one.
	Tabs []PageTab `yaml:"tabs" json:"tabs"`
}

// PageTab is one tab of a [Page]: a navigation destination with an id for
// the URL, `/p/<page>/<tab>`.
//
// Exactly one of List, Kanban, Calendar, Gantt, Dashboard and Document is
// set. Enforced by validatePages.
type PageTab struct {
	// ID names the tab in the URL. It must match [urlSegmentPattern] and be
	// unique within the page.
	ID string `yaml:"id" json:"id"`
	// Label is the tab's text. Required.
	Label string `yaml:"label" json:"label"`
	// Icon is an optional glyph name.
	Icon string `yaml:"icon,omitempty" json:"icon,omitempty"`
	// Permission hides the tab from principals who do not hold the named
	// global ACL permission. It has the semantics of
	// [NavigationEntry.Permission]: a UX filter, not access control.
	Permission string `yaml:"permission,omitempty" json:"permission,omitempty"`

	List      string `yaml:"list,omitempty" json:"list,omitempty"`
	Kanban    string `yaml:"kanban,omitempty" json:"kanban,omitempty"`
	Calendar  string `yaml:"calendar,omitempty" json:"calendar,omitempty"`
	Gantt     string `yaml:"gantt,omitempty" json:"gantt,omitempty"`
	Dashboard bool   `yaml:"dashboard,omitempty" json:"dashboard,omitempty"`
	// Document names a standalone document, as on a navigation entry.
	Document string `yaml:"document,omitempty" json:"document,omitempty"`

	// Scope narrows the tab to the page's anchor entity. Required on every
	// tab of an entity page, and refused on any other page.
	//
	// Nil: accepted on a page without entity_type, where it means "unscoped".
	Scope *PageTabScope `yaml:"scope,omitempty" json:"scope,omitempty"`
}

// PageTabScope says how a tab of an entity page narrows to the anchor.
//
// Written as `scope: root` or as a mapping `scope: {relation, direction}`:
//   - Relation (list and kanban tabs) keeps the rows the anchor reaches over
//     that relation. Direction is from the anchor's point of view, as in view
//     sections: outgoing means anchor --relation--> row. Left out, it is
//     inferred from the metamodel like every other relation binding.
//   - Root (gantt tabs) draws the timeline from the anchor down, as a drill
//     into the anchor does.
//
// The scope is configuration only. A request names a page and a tab, never a
// relation, so a caller cannot make a list narrow by something the operator
// did not declare.
type PageTabScope struct {
	Root      bool      `yaml:"-" json:"root,omitempty"`
	Relation  string    `yaml:"relation,omitempty" json:"relation,omitempty"`
	Direction Direction `yaml:"direction,omitempty" json:"direction,omitempty"`
}

// PageScopeRoot is the scalar form of a gantt tab's scope.
const PageScopeRoot = "root"

// UnmarshalYAML accepts `root` and the relation mapping.
func (s *PageTabScope) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		if strings.TrimSpace(value.Value) != PageScopeRoot {
			return fmt.Errorf("invalid scope at line %d: %q (must be %q or a mapping with relation and direction)",
				value.Line, value.Value, PageScopeRoot)
		}
		*s = PageTabScope{Root: true}
		return nil
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			if key := value.Content[i].Value; key != "relation" && key != "direction" {
				return fmt.Errorf("invalid scope at line %d: unknown key %q (valid: direction, relation)",
					value.Content[i].Line, key)
			}
		}
		// Alias to avoid recursing into this method.
		type rawScope PageTabScope
		var raw rawScope
		if err := value.Decode(&raw); err != nil {
			return fmt.Errorf("invalid scope: %w", err)
		}
		*s = PageTabScope(raw)
		return nil
	default:
		return fmt.Errorf("invalid scope at line %d: got %s, must be %q or a mapping",
			value.Line, yamlKindName(value.Kind), PageScopeRoot)
	}
}

// ResolvedDirection returns the scope's direction, inferred from the
// metamodel when the config leaves it out. anchorType is the page's
// entity_type. An ambiguous or unresolvable relation is a load error, so on a
// validated config the inferred answer is the only one.
func (s PageTabScope) ResolvedDirection(anchorType string, meta *metamodel.Metamodel) Direction {
	if s.Direction != "" {
		return s.Direction
	}
	dir, _ := InferDirection(anchorType, s.Relation, meta)
	return dir
}

// IsEntityPage reports whether the page shows one entity (see [Page]).
func (p Page) IsEntityPage() bool {
	return p.EntityType != ""
}

// Tab returns the tab with the given id.
func (p Page) Tab(id string) (PageTab, bool) {
	for _, t := range p.Tabs {
		if t.ID == id {
			return t, true
		}
	}
	return PageTab{}, false
}

// Destination returns the tab as a navigation entry, so the checks and
// conversions written for a navigation destination apply to a tab unchanged.
func (t PageTab) Destination() NavigationEntry {
	return NavigationEntry{
		Label:      t.Label,
		Icon:       t.Icon,
		Permission: t.Permission,
		List:       t.List,
		Kanban:     t.Kanban,
		Calendar:   t.Calendar,
		Gantt:      t.Gantt,
		Dashboard:  t.Dashboard,
		Document:   t.Document,
	}
}

// destinationCount is the number of destination keys the tab sets.
func (t PageTab) destinationCount() int {
	n := 0
	for _, set := range []bool{
		t.List != "", t.Kanban != "", t.Calendar != "", t.Gantt != "", t.Dashboard, t.Document != "",
	} {
		if set {
			n++
		}
	}
	return n
}

// urlSegmentPattern is the shape of a page id and a tab id: short,
// lowercase and safe as a URL path segment. The same shape as a space id.
var urlSegmentPattern = spaceIDPattern

// pageTabDestinations names the keys a tab may set, for error messages.
const pageTabDestinations = "list, kanban, calendar, gantt, dashboard or document"

// NavEntryList returns the list whose rows a navigation entry's `status:`
// rules count: the entry's own list, or for a `page:` entry the list of the
// page's first tab. Empty when there is none.
func (c *Config) NavEntryList(entry NavigationEntry) string {
	if entry.List != "" || entry.Page == "" || c == nil {
		return entry.List
	}
	page, ok := c.Pages[entry.Page]
	if !ok || len(page.Tabs) == 0 {
		return ""
	}
	return page.Tabs[0].List
}

// validatePages checks `pages:`. Every error names the page, and the tab
// where there is one, so an author can find the line.
func validatePages(cfg *Config, meta *metamodel.Metamodel) []string {
	var errs []string
	for _, id := range sortedMapKeys(cfg.Pages) {
		page := cfg.Pages[id]
		name := fmt.Sprintf("pages[%s]", id)
		if !urlSegmentPattern.MatchString(id) {
			errs = append(errs, fmt.Sprintf("%s: invalid id %q (must match %s)", name, id, urlSegmentPattern))
		}
		if strings.TrimSpace(page.Label) == "" {
			errs = append(errs, name+": label is required")
		}
		errs = append(errs, validateIconName(page.Icon, name)...)
		if len(page.Tabs) == 0 {
			errs = append(errs, name+": tabs is required (at least one tab)")
		}
		errs = append(errs, validateEntityPage(name, page, meta)...)
		seen := make(map[string]bool, len(page.Tabs))
		for i, tab := range page.Tabs {
			errs = append(errs, validatePageTab(cfg, name, i, tab, seen)...)
			errs = append(errs, validatePageTabScope(cfg, meta, name, page, i, tab)...)
		}
	}
	return errs
}

// validateEntityPage checks a page's entity_type and badge.
func validateEntityPage(name string, page Page, meta *metamodel.Metamodel) []string {
	if !page.IsEntityPage() {
		if page.Badge != "" {
			return []string{name + ": badge needs entity_type (only an entity page has an entity to show)"}
		}
		return nil
	}
	if meta == nil {
		return nil
	}
	def, ok := meta.GetEntityDef(page.EntityType)
	if !ok {
		return []string{fmt.Sprintf("%s: unknown entity_type %q", name, page.EntityType)}
	}
	if page.Badge != "" {
		if _, ok := def.Properties[page.Badge]; !ok {
			return []string{fmt.Sprintf("%s: badge property %q not in metamodel for entity %q",
				name, page.Badge, page.EntityType)}
		}
	}
	return nil
}

// validatePageTabScope checks a tab's scope against its page. An entity page
// needs every tab scoped, because an unscoped tab would show the whole type
// under one entity's title. Any other page has no anchor to scope to.
func validatePageTabScope(
	cfg *Config, meta *metamodel.Metamodel, pageName string, page Page, i int, tab PageTab,
) []string {
	name := fmt.Sprintf("%s.tabs[%s]", pageName, tab.ID)
	if tab.ID == "" || !urlSegmentPattern.MatchString(tab.ID) {
		name = fmt.Sprintf("%s.tabs[%d]", pageName, i)
	}
	if !page.IsEntityPage() {
		if tab.Scope != nil {
			return []string{name + ": scope needs entity_type on the page (only an entity page has an anchor)"}
		}
		return nil
	}
	switch {
	case tab.List != "" || tab.Kanban != "":
		return validateRelationScope(cfg, meta, name, page.EntityType, tab)
	case tab.Gantt != "":
		return validateRootScope(cfg, meta, name, page.EntityType, tab)
	case tab.destinationCount() == 0:
		return nil // reported by validatePageTab
	default:
		return []string{name + ": an entity page can only show list, kanban and gantt tabs " +
			"(a calendar, document or dashboard cannot be scoped to the page's entity)"}
	}
}

// validateRelationScope checks the relation scope of a list or kanban tab:
// the relation must connect the anchor type to the tab's type in the scope's
// direction, or the tab would always be empty.
func validateRelationScope(cfg *Config, meta *metamodel.Metamodel, name, anchorType string, tab PageTab) []string {
	sc := tab.Scope
	switch {
	case sc == nil:
		return []string{name + ": scope is required on an entity page (set scope: {relation, direction})"}
	case sc.Root:
		return []string{name + ": scope: root is only for a gantt tab (a list or kanban needs a relation)"}
	case sc.Relation == "":
		return []string{name + ": scope.relation is required"}
	}
	rowType := cfg.Lists[tab.List].EntityType
	if tab.Kanban != "" {
		rowType = cfg.Kanbans[tab.Kanban].EntityType
	}
	if meta == nil || rowType == "" {
		return nil // an unknown view is reported by validatePageTab
	}
	def, ok := meta.GetRelationDef(sc.Relation)
	if !ok {
		return []string{fmt.Sprintf("%s: scope.relation %q not in metamodel", name, sc.Relation)}
	}
	if errs := CheckAmbiguousDirection(name+": scope", anchorType, sc.Relation, sc.Direction, meta); errs != nil {
		return errs
	}
	anchorSide, rowSide := def.From, def.To
	if sc.ResolvedDirection(anchorType, meta).IsIncoming() {
		anchorSide, rowSide = def.To, def.From
	}
	if !slices.Contains(anchorSide, anchorType) || !slices.Contains(rowSide, rowType) {
		return []string{fmt.Sprintf(
			"%s: scope.relation %q does not connect %q to %q %s (from: %s; to: %s)",
			name, sc.Relation, anchorType, rowType, sc.ResolvedDirection(anchorType, meta),
			strings.Join(def.From, ", "), strings.Join(def.To, ", "))}
	}
	return nil
}

// validateRootScope checks a gantt tab's `scope: root`. The gantt draws the
// anchor as its root, so the anchor type must be one of its sources and the
// parent side of a hierarchy relation; otherwise every anchor answers 404.
func validateRootScope(cfg *Config, meta *metamodel.Metamodel, name, anchorType string, tab PageTab) []string {
	if tab.Scope == nil || !tab.Scope.Root {
		return []string{name + ": a gantt tab on an entity page needs scope: root"}
	}
	g, ok := cfg.Gantts[tab.Gantt]
	if !ok {
		return nil // reported by validatePageTab
	}
	if _, ok := g.Sources[anchorType]; !ok {
		return []string{fmt.Sprintf("%s: gantt %q has no source for %q, so it cannot start at the page's entity",
			name, tab.Gantt, anchorType)}
	}
	if meta == nil {
		return nil
	}
	for _, rel := range g.Hierarchy {
		if def, ok := meta.GetRelationDef(rel); ok && slices.Contains(def.From, anchorType) {
			return nil
		}
	}
	return []string{fmt.Sprintf("%s: no hierarchy relation of gantt %q runs from %q, so the timeline "+
		"under the page's entity would always be empty", name, tab.Gantt, anchorType)}
}

// validatePageTab checks one tab. seen collects the tab ids of the page.
func validatePageTab(cfg *Config, page string, i int, tab PageTab, seen map[string]bool) []string {
	var errs []string
	name := fmt.Sprintf("%s.tabs[%s]", page, tab.ID)
	switch {
	case tab.ID == "":
		name = fmt.Sprintf("%s.tabs[%d]", page, i)
		errs = append(errs, name+": id is required")
	case !urlSegmentPattern.MatchString(tab.ID):
		name = fmt.Sprintf("%s.tabs[%d]", page, i)
		errs = append(errs, fmt.Sprintf("%s: invalid id %q (must match %s)", name, tab.ID, urlSegmentPattern))
	case seen[tab.ID]:
		errs = append(errs, name+": duplicate id")
	}
	seen[tab.ID] = true

	if strings.TrimSpace(tab.Label) == "" {
		errs = append(errs, name+": label is required")
	}
	errs = append(errs, validateIconName(tab.Icon, name)...)
	switch tab.destinationCount() {
	case 0:
		errs = append(errs, fmt.Sprintf("%s: names no view (set one of %s)", name, pageTabDestinations))
	case 1:
	default:
		errs = append(errs, fmt.Sprintf("%s: names more than one view (set one of %s)", name, pageTabDestinations))
	}
	for _, e := range navTargetErrors(tab.Destination(), cfg) {
		errs = append(errs, name+": "+e)
	}
	return errs
}

// validatePageStatus checks that a `page:` entry with `status:` rules has a
// list to count: its page's first tab. label names the entry.
func validatePageStatus(nav NavigationEntry, label string, cfg *Config) []string {
	page, ok := cfg.Pages[nav.Page]
	if !ok {
		return nil // the unknown page is reported by navTargetErrors
	}
	if len(page.Tabs) == 0 || page.Tabs[0].List == "" {
		return []string{fmt.Sprintf(
			"navigation %q: status on a page entry needs the page's first tab to be a list "+
				"(pages[%s].tabs[0] is not)", label, nav.Page)}
	}
	return nil
}
