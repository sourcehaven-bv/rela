package configedit

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// File names the configuration file a tree belongs to.
type File string

// The two files the Configure space edits.
const (
	SchemaFile    File = "schema.yaml"
	DataEntryFile File = "data-entry.yaml"
)

// The allowlist says which keys the Configure space may create or change.
//
// It is per key, not per section, because the keys that run code sit deep
// inside sections that are otherwise safe to edit: a file property's
// scan_cmd, a validation's lua, an automation action's capabilities. A
// principal who may edit forms must not thereby gain a way to run commands.
//
// A pattern is a dot-separated key path. "*" matches any one key, "[]" any
// list item, and a trailing "**" anything below. Every key a configuration
// struct declares must be classified by some pattern here, editable or locked;
// TestAllowlist_ClassifiesEveryKey walks the structs and fails on a key that
// is not, so a new key added to the metamodel is locked until someone
// decides otherwise. A key that matches no pattern at all is locked too.
//
// Removing a whole item (a property, a form, an automation) is allowed when
// the item holds anything editable, even if it also holds locked keys:
// removal takes code away, it never adds it. The exception is an item holding
// a key listed in protective, and removing or changing one locked key on its
// own is refused.
var editable = map[File][]string{
	SchemaFile: {
		"description",

		"types.*.default",
		"types.*.description",
		"types.*.descriptions.*",
		"types.*.initial",
		"types.*.labels.*",
		"types.*.values.[]",
		"types.*.transitions.[].from",
		"types.*.transitions.[].to",
		"types.*.transitions.[].label",
		"types.*.transitions.[].help",
		"types.*.transitions.[].when",
		"types.*.validations.[].error",
		"types.*.validations.[].pattern",

		"entities.*.aliases.[]",
		// Named membership predicates: an expression, compiled on save.
		"entities.*.query_scopes.*",
		"entities.*.rdf_type",
		"comments.**",
		"entities.*.border_color",
		"entities.*.color",
		"entities.*.default_sort.[].direction",
		"entities.*.default_sort.[].property",
		"entities.*.description",
		"entities.*.display_property",
		"entities.*.id_caps",
		"entities.*.id_prefix",
		"entities.*.id_prefixes.[]",
		"entities.*.id_type",
		"entities.*.label",
		"entities.*.label_plural",
		"entities.*.plural",
		"entities.*.properties.*.default",
		"entities.*.properties.*.description",
		"entities.*.properties.*.format",
		"entities.*.properties.*.labels.*",
		"entities.*.properties.*.list",
		"entities.*.properties.*.max",
		"entities.*.properties.*.required",
		"entities.*.properties.*.type",
		"entities.*.properties.*.unique",
		"entities.*.properties.*.values.[]",

		"relations.*.content",
		"relations.*.description",
		"relations.*.from.[]",
		"relations.*.to.[]",
		"relations.*.inverse", // the short form, a bare name
		"relations.*.inverse.id",
		"relations.*.inverse.label",
		"relations.*.label",
		"relations.*.max_incoming",
		"relations.*.max_outgoing",
		"relations.*.min_incoming",
		"relations.*.min_outgoing",
		"relations.*.orderable",
		"relations.*.symmetric",
		"relations.*.properties.*.default",
		"relations.*.properties.*.description",
		"relations.*.properties.*.format",
		"relations.*.properties.*.labels.*",
		"relations.*.properties.*.list",
		"relations.*.properties.*.max",
		"relations.*.properties.*.required",
		"relations.*.properties.*.type",
		"relations.*.properties.*.unique",
		"relations.*.properties.*.values.[]",

		"validations.[].content.**",
		"validations.[].description",
		"validations.[].entity_type",
		"validations.[].name",
		"validations.[].relations.*.direction",
		"validations.[].relations.*.max",
		"validations.[].relations.*.min",
		"validations.[].relations.*.target_type",
		"validations.[].relations.*.where.[]",
		"validations.[].severity",
		"validations.[].then.[]",
		"validations.[].then_condition",
		"validations.[].when.[]",
		"validations.[].when_condition",

		"automations.[].description",
		"automations.[].name",
		"automations.[].on.becomes",
		"automations.[].on.condition",
		"automations.[].on.created",
		"automations.[].on.entity.[]",
		"automations.[].on.from",
		"automations.[].on.property",
		"automations.[].on.relation_created",
		"automations.[].on.relation_removed",
		"automations.[].on.when.[]",
		"automations.[].do.[].set",
		"automations.[].do.[].value",
		"automations.[].do.[].create_relation.relation",
		"automations.[].do.[].create_relation.to",
		"automations.[].do.[].create_entity.if_exists",
		"automations.[].do.[].create_entity.properties.*",
		"automations.[].do.[].create_entity.relation",
		"automations.[].do.[].create_entity.type",
		"automations.[].validate.[].check",
		"automations.[].validate.[].message",
		"automations.[].validate.[].severity",
	},
	DataEntryFile: {
		// Detail views, calendars and timelines choose which properties and
		// relations a page shows. The data behind them stays ACL-gated, and
		// their markdown headers are sanitized when shown. A view's
		// export_render runs Lua and stays locked.
		"views.*.title", "views.*.entry.**", "views.*.traverse.**", "views.*.sections.**",
		"entity_views.**", "detail_view", "lists.*.detail_view", "duplicate.**",
		"forms.*.side_panel.**", "calendars.**", "gantts.**",
		"app.name",
		"app.description",

		"styles.*.*",

		"forms.*.title",
		"forms.*.description",
		"forms.*.entity_type",
		"forms.*.mode",
		"forms.*.body",
		"forms.*.fields.[].**",
		"forms.*.relations.[].**",
		"forms.*.steps.[].**",

		"lists.*.columns.[].direction",
		"lists.*.columns.[].label",
		"lists.*.columns.[].link",
		"lists.*.columns.[].property",
		"lists.*.columns.[].relation",
		"lists.*.columns.[].sortable",
		"lists.*.condition",
		"lists.*.create_form",
		"lists.*.description",
		"lists.*.edit_form",
		"lists.*.entity_type",
		"lists.*.filter_controls.[].**",
		"lists.*.filters.[].**",
		"lists.*.footer",
		"lists.*.group_by.**",
		"lists.*.header",
		"lists.*.page_size",
		"lists.*.query_scope",
		"lists.*.sort.[].**",
		"lists.*.title",

		"kanbans.*.card.**",
		"kanbans.*.column_property",
		"kanbans.*.columns.[].**",
		"kanbans.*.condition",
		"kanbans.*.create_form",
		"kanbans.*.edit_form",
		"kanbans.*.entity_type",
		"kanbans.*.filter_controls.[].**",
		"kanbans.*.filters.[].**",
		"kanbans.*.footer",
		"kanbans.*.header",
		"kanbans.*.query_scope",
		"kanbans.*.swimlane_property",
		"kanbans.*.swimlanes.[].**",
		"kanbans.*.title",

		"dashboard.title",
		"dashboard.description",
		"dashboard.cards.[].columns.[].direction",
		"dashboard.cards.[].columns.[].label",
		"dashboard.cards.[].columns.[].link",
		"dashboard.cards.[].columns.[].property",
		"dashboard.cards.[].columns.[].relation",
		"dashboard.cards.[].columns.[].sortable",
		"dashboard.cards.[].display",
		"dashboard.cards.[].group_by",
		"dashboard.cards.[].limit",
		"dashboard.cards.[].query",
		"dashboard.cards.[].sort.[].**",
		"dashboard.cards.[].title",

		"spaces.[].id",
		"spaces.[].label",
		"spaces.[].icon",
		"spaces.[].create.[]",

		// A navigation entry, wherever it sits (see navPrefixes).
		"nav.calendar",
		"nav.collapsed",
		"nav.dashboard",
		"nav.document",
		"nav.entities",
		"nav.gantt",
		"nav.group",
		"nav.icon",
		"nav.items_from.**",
		"nav.kanban",
		"nav.label",
		"nav.list",
		"nav.open",
		"nav.page",
		"nav.query_scope",
		"nav.search",
		"nav.settings",
		"nav.sort.[].**",
		"nav.status.[].**",
	},
}

// locked lists the keys deliberately left out of the Configure space, so the
// guard test can tell a decision from an oversight. Locking does not depend
// on this list: anything not editable is locked.
var locked = map[File][]string{
	SchemaFile: {
		"version", "namespace", "includes.**",
		// External commands and upload policy.
		"attachments.**", "transforms.**",
		"entities.*.properties.*.accept.**",
		"entities.*.properties.*.scan",
		"entities.*.properties.*.scan_cmd.**",
		"entities.*.properties.*.transform.**",
		"relations.*.properties.*.accept.**",
		"relations.*.properties.*.scan",
		"relations.*.properties.*.scan_cmd.**",
		"relations.*.properties.*.transform.**",
		// Lua, and what a script may reach.
		"validations.[].lua", "validations.[].lua_args.**", "validations.[].lua_file",
		"automations.[].do.[].lua", "automations.[].do.[].lua_file",
		"automations.[].do.[].allow_acl_bypass", "automations.[].do.[].capabilities.**",
		"automations.[].do.[].create_entity.template",
		// Access control: a transition guard names an ACL permission.
		"types.*.transitions.[].guard",
		// Not in the Configure space yet.
		"copies.**", "worlds.**", "entities.*.faces.**",
		"entities.*.properties.*.computed", "relations.*.properties.*.computed",
		"relations.*.scope",
		"validations.[].faces.**", "automations.[].on.faces.**",
	},
	DataEntryFile: {
		"version",
		"app.default_world", "app.disable_custom_injection", "app.max_attachment_bytes", "app.plantuml_server_url",
		// Access control and actions.
		"nav.permission", "nav.action", "spaces.[].permission", "dashboard.cards.[].permission",
		"lists.*.actions.**", "lists.*.export_render",
		// Not in the Configure space yet.
		"views.*.export_render",
		"git.**", "palette.**",
		"documents.**", "feeds.**", "caldav.**", "commands.**", "actions.**", "webhooks.**",
		"pages.**", "next_action_bands.**", "next_actions.**", "account.**", "lists.*.create_world",
		"lists.*.columns.[].face", "dashboard.cards.[].columns.[].face",
	},
}

// protective lists the locked keys that restrict rather than run code: a
// guard, a permission, an upload check, a limit. Removing an item that holds
// one is refused, because removing it and adding it back without the key
// would loosen the restriction through two allowed steps.
var protective = map[File][]string{
	SchemaFile: {
		"attachments.**",
		"entities.*.properties.*.accept.**",
		"entities.*.properties.*.scan",
		"entities.*.properties.*.scan_cmd.**",
		"relations.*.properties.*.accept.**",
		"relations.*.properties.*.scan",
		"relations.*.properties.*.scan_cmd.**",
		"types.*.transitions.[].guard",
	},
	DataEntryFile: {
		"app.disable_custom_injection", "app.max_attachment_bytes",
		"nav.permission", "spaces.[].permission", "dashboard.cards.[].permission",
	},
}

// A pin keeps the editable keys of an item that say what its locked keys
// apply to from changing while the item holds one of them.
type pin struct {
	item string   // pattern of the item
	keys []string // its keys that cannot change
	by   []string // patterns of the locked keys that pin them
}

// pinned lists the pins. Moving a guarded transition elsewhere would free
// the edge it guarded, and adding that edge back without a guard is an
// ordinary edit. (A second edge with the same from and to is refused by the
// state machine itself.) Pointing reviewed Lua, or an action that may reach
// past the ACL, at other records is a new use of that code, so the trigger
// and scope of an item holding it stay as they are.
//
// The navigation, space and dashboard permission: keys need no pin: they
// only decide what the menu shows, and the data behind an entry stays
// ACL-gated whatever it points at.
var pinned = map[File][]pin{
	SchemaFile: {
		{item: "types.*.transitions.[]", keys: []string{"from", "to"}, by: []string{"types.*.transitions.[].guard"}},
		{
			item: "validations.[]",
			keys: []string{"entity_type", "when", "when_condition"},
			by:   []string{"validations.[].lua", "validations.[].lua_file", "validations.[].lua_args.**"},
		},
		{
			item: "automations.[]",
			keys: []string{"on"},
			by: []string{
				"automations.[].do.[].lua", "automations.[].do.[].lua_file",
				"automations.[].do.[].allow_acl_bypass", "automations.[].do.[].capabilities.**",
			},
		},
	},
}

// holdsProtective reports whether v, found at path, is or contains a key
// listed in protective.
func holdsProtective(file File, path []string, v any) bool {
	return holdsAny(file, path, v, protective[file])
}

// holdsAny reports whether v, found at path, is or contains a key matching
// one of patterns.
func holdsAny(file File, path []string, v any, patterns []string) bool {
	if anyMatch(patterns, normalize(file, path), match) {
		return true
	}
	switch t := v.(type) {
	case *Map:
		for i, k := range t.Keys {
			if holdsAny(file, with(path, k), t.Values[i], patterns) {
				return true
			}
		}
	case []any:
		for _, item := range t {
			if holdsAny(file, with(path, "[]"), item, patterns) {
				return true
			}
		}
	}
	return false
}

// navPrefixes are the places a navigation entry sits. An entry's sub-items
// are entries again, to any depth, so every entry path is rewritten to "nav".
var navPrefixes = [][]string{
	{"navigation", "[]"},
	{"spaces", "[]", "navigation", "[]"},
	{"spaces", "[]", "home"},
}

// normalize turns a path of keys and list markers into the form patterns are
// written in.
func normalize(file File, path []string) []string {
	if file != DataEntryFile {
		return path
	}
	for _, prefix := range navPrefixes {
		if !hasPrefix(path, prefix) {
			continue
		}
		rest := path[len(prefix):]
		for len(rest) >= 2 && rest[0] == "items" && rest[1] == "[]" {
			rest = rest[2:]
		}
		return append([]string{"nav"}, rest...)
	}
	return path
}

func hasPrefix(path, prefix []string) bool {
	if len(path) < len(prefix) {
		return false
	}
	for i, p := range prefix {
		if path[i] != p {
			return false
		}
	}
	return true
}

// match reports whether a pattern matches a whole path. A pattern's "[]" may
// also match nothing, because several keys take either one value or a list
// of them (`entity: ticket` or `entity: [ticket, bug]`).
func match(pattern, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	switch pattern[0] {
	case "**":
		return true
	case "[]":
		if len(path) > 0 && path[0] == "[]" && match(pattern[1:], path[1:]) {
			return true
		}
		return match(pattern[1:], path)
	}
	if len(path) == 0 || path[0] == "[]" {
		return false
	}
	if pattern[0] != "*" && pattern[0] != path[0] {
		return false
	}
	return match(pattern[1:], path[1:])
}

// prefixOf reports whether a path is a container some pattern reaches into.
func prefixOf(pattern, path []string) bool {
	if len(path) == 0 {
		return true
	}
	if len(pattern) == 0 {
		return false
	}
	switch pattern[0] {
	case "**":
		return true
	case "[]":
		if path[0] == "[]" && prefixOf(pattern[1:], path[1:]) {
			return true
		}
		return prefixOf(pattern[1:], path)
	}
	if path[0] == "[]" || (pattern[0] != "*" && pattern[0] != path[0]) {
		return false
	}
	return prefixOf(pattern[1:], path[1:])
}

func anyMatch(patterns, path []string, fn func(pattern, path []string) bool) bool {
	for _, p := range patterns {
		if fn(strings.Split(p, "."), path) {
			return true
		}
	}
	return false
}

func isEditable(file File, path []string) bool {
	return anyMatch(editable[file], normalize(file, path), match)
}

// holdsEditable reports whether something under path is editable, which is
// what makes removing the whole of it allowed.
func holdsEditable(file File, path []string) bool {
	return anyMatch(editable[file], normalize(file, path), prefixOf)
}

// A Violation is an edit the Configure space may not make.
type Violation struct {
	File File   `json:"file"`
	Path string `json:"path"`
	// Change is "added", "changed" or "removed".
	Change string `json:"change"`
}

// CheckEdit compares an edited tree with the file's current tree and returns
// every change the allowlist refuses, in path order.
func CheckEdit(file File, before, after any) []Violation {
	c := checker{file: file}
	c.diff(nil, nil, before, after)
	sort.Slice(c.out, func(i, j int) bool { return c.out[i].Path < c.out[j].Path })
	return c.out
}

type checker struct {
	file File
	out  []Violation
}

func (c *checker) refuse(display []string, change string) {
	c.out = append(c.out, Violation{File: c.file, Path: strings.Join(display, "."), Change: change})
}

// checkPinned refuses a change to a pinned key of an item that held a key
// pinning it before the edit.
func (c *checker) checkPinned(path, display []string, before, after *Map) {
	for _, p := range pinned[c.file] {
		if !match(strings.Split(p.item, "."), normalize(c.file, path)) || !holdsAny(c.file, path, before, p.by) {
			continue
		}
		for _, k := range p.keys {
			old, _ := before.Get(k)
			now, _ := after.Get(k)
			if !reflect.DeepEqual(stripped(old), stripped(now)) {
				c.refuse(with(display, k), "changed")
			}
		}
	}
}

// diff walks both trees together. path is the pattern form (list items as
// "[]"); display is the same path with list indexes, for messages.
func (c *checker) diff(path, display []string, before, after any) {
	bm, bIsMap := before.(*Map)
	am, aIsMap := after.(*Map)
	if bIsMap && aIsMap {
		c.checkPinned(path, display, bm, am)
		for i, k := range am.Keys {
			old, ok := bm.Get(k)
			if !ok {
				c.added(with(path, k), with(display, k), am.Values[i])
				continue
			}
			c.diff(with(path, k), with(display, k), old, am.Values[i])
		}
		for i, k := range bm.Keys {
			if _, ok := am.Get(k); !ok {
				c.removed(with(path, k), with(display, k), bm.Values[i])
			}
		}
		return
	}
	bl, bIsList := before.([]any)
	al, aIsList := after.([]any)
	if bIsList && aIsList {
		c.diffList(path, display, bl, al)
		return
	}
	if reflect.DeepEqual(stripped(before), stripped(after)) {
		return
	}
	if isEditable(c.file, path) {
		// An editable value may change freely, but a mapping or list put
		// in its place brings keys of its own, and those are checked.
		if isContainer(after) && !isContainer(before) {
			c.added(path, display, after)
		}
		return
	}
	if before == nil {
		c.added(path, display, after)
		return
	}
	if after == nil {
		c.removed(path, display, before)
		return
	}
	if !isContainer(before) && !isContainer(after) {
		c.refuse(display, "changed")
		return
	}
	c.removed(path, display, before)
	c.added(path, display, after)
}

// diffList pairs items by the index they were read at; an item without one
// is new, and an old item nothing points at was removed. A list of plain
// values is one value as far as the allowlist is concerned.
func (c *checker) diffList(path, display []string, before, after []any) {
	itemPath := with(path, "[]")
	if scalarList(before) && scalarList(after) {
		if !reflect.DeepEqual(before, after) && !isEditable(c.file, itemPath) {
			c.refuse(display, "changed")
		}
		return
	}
	claimed := make([]bool, len(before))
	for i, item := range after {
		at := with(display, strconv.Itoa(i))
		if m, ok := item.(*Map); ok && m.HasOrigin && m.Origin < len(before) && !claimed[m.Origin] {
			claimed[m.Origin] = true
			c.diff(itemPath, at, before[m.Origin], item)
			continue
		}
		c.added(itemPath, at, item)
	}
	for i, item := range before {
		if !claimed[i] {
			c.removed(itemPath, with(display, strconv.Itoa(i)), item)
		}
	}
}

// added checks every key a new value brings.
func (c *checker) added(path, display []string, v any) {
	switch t := v.(type) {
	case *Map:
		if len(t.Keys) == 0 && !holdsEditable(c.file, path) {
			c.refuse(display, "added")
		}
		for i, k := range t.Keys {
			c.added(with(path, k), with(display, k), t.Values[i])
		}
	case []any:
		if scalarList(t) {
			if !isEditable(c.file, with(path, "[]")) {
				c.refuse(display, "added")
			}
			return
		}
		for i, item := range t {
			c.added(with(path, "[]"), with(display, strconv.Itoa(i)), item)
		}
	default:
		if !isEditable(c.file, path) {
			c.refuse(display, "added")
		}
	}
}

func (c *checker) removed(path, display []string, v any) {
	if holdsProtective(c.file, path, v) {
		c.refuse(display, "removed")
		return
	}
	if isEditable(c.file, path) {
		return
	}
	switch v.(type) {
	case *Map, []any:
		if holdsEditable(c.file, path) {
			return
		}
	}
	c.refuse(display, "removed")
}

// maxChangedPaths caps how many paths changedPaths reports, so one large
// edit cannot make an audit record unbounded.
const maxChangedPaths = 50

// changedPaths lists where after differs from before, as dotted paths: the
// deepest mapping key whose value differs, with a list counted as one value.
// It ends in "…" when more than maxChangedPaths differ.
func changedPaths(before, after any, path []string) []string {
	bm, bIsMap := before.(*Map)
	am, aIsMap := after.(*Map)
	if !bIsMap || !aIsMap {
		if reflect.DeepEqual(stripped(before), stripped(after)) {
			return nil
		}
		return []string{strings.Join(path, ".")}
	}
	var out []string
	add := func(paths []string) {
		for _, p := range paths {
			if len(out) == maxChangedPaths {
				out = append(out, "…")
			}
			if len(out) > maxChangedPaths {
				return
			}
			out = append(out, p)
		}
	}
	for i, k := range am.Keys {
		old, _ := bm.Get(k)
		add(changedPaths(old, am.Values[i], with(path, k)))
	}
	for _, k := range bm.Keys {
		if _, ok := am.Get(k); !ok {
			add([]string{strings.Join(with(path, k), ".")})
		}
	}
	return out
}

func isContainer(v any) bool {
	switch v.(type) {
	case *Map, []any:
		return true
	}
	return false
}

func scalarList(items []any) bool {
	for _, item := range items {
		switch item.(type) {
		case *Map, []any:
			return false
		}
	}
	return true
}

// stripped drops list origins, which differ between two reads of the same
// content once an item before them was removed.
func stripped(v any) any {
	switch t := v.(type) {
	case *Map:
		out := &Map{Keys: t.Keys, Values: make([]any, len(t.Values))}
		for i, c := range t.Values {
			out.Values[i] = stripped(c)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, c := range t {
			out[i] = stripped(c)
		}
		return out
	}
	return v
}

func with(path []string, key string) []string {
	out := make([]string, len(path), len(path)+1)
	copy(out, path)
	return append(out, key)
}
