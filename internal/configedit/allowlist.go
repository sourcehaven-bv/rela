package configedit

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// File names the configuration file a tree belongs to.
type File string

// The files the Configure space edits. flows.yaml is open for its
// `triggers:` only (TKT-ZT2KREN).
const (
	SchemaFile    File = "schema.yaml"
	DataEntryFile File = "data-entry.yaml"
	FlowsFile     File = "flows.yaml"
)

// The allowlist says which keys the Configure space may create or change.
//
// It is per key, not per section, because the keys that run code sit deep
// inside sections that are otherwise safe to edit: a file property's
// scan_cmd, a validation's lua. A principal who may edit forms must not
// thereby gain a way to run commands.
//
// A pattern is a dot-separated key path. "*" matches any one key, "[]" any
// list item, and a trailing "**" anything below. Every key a configuration
// struct declares must be classified by some pattern here, editable or locked;
// TestAllowlist_ClassifiesEveryKey walks the structs and fails on a key that
// is not, so a new key added to the metamodel is locked until someone
// decides otherwise. A key that matches no pattern at all is locked too.
//
// Removing a whole item (a property, a form, a validation) is allowed when
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
		"entities.*.properties.*.display.currency",
		"entities.*.properties.*.display.link",
		"entities.*.properties.*.display.link_text",
		"entities.*.properties.*.display.image",
		"entities.*.properties.*.label",
		"entities.*.properties.*.format",
		"entities.*.properties.*.labels.*",
		"entities.*.properties.*.list",
		"entities.*.properties.*.max",
		"entities.*.properties.*.min",
		"entities.*.properties.*.required",
		"entities.*.properties.*.scale",
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
		"relations.*.properties.*.display.currency",
		"relations.*.properties.*.display.link",
		"relations.*.properties.*.display.link_text",
		"relations.*.properties.*.display.image",
		"relations.*.properties.*.label",
		"relations.*.properties.*.format",
		"relations.*.properties.*.labels.*",
		"relations.*.properties.*.list",
		"relations.*.properties.*.max",
		"relations.*.properties.*.min",
		"relations.*.properties.*.required",
		"relations.*.properties.*.scale",
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

		// Declared piles (TKT-EO2GCM) name, decorate and expire per-user lists
		// of refs; they run no code and grant no read.
		"piles.**",
	},
	FlowsFile: {
		// Entity-change triggers (TKT-ZT2KREN): the keys the schema.yaml
		// automations allowlist opened before TKT-GWOBH2U moved them here.
		// An output writes as a cascade, without a per-record ACL check, so
		// every edit is also judged against acl.yaml (see TriggerChecker).
		"triggers.*.description",
		"triggers.*.on.becomes",
		"triggers.*.on.created",
		"triggers.*.on.entity.[]",
		"triggers.*.on.from",
		"triggers.*.on.property",
		"triggers.*.on.relation_created",
		"triggers.*.on.relation_removed",
		"triggers.*.on.updated",
		"triggers.*.on.when",
		"triggers.*.on.where.[]",
		"triggers.*.outputs.[].set.properties.*",
		"triggers.*.outputs.[].create.properties.*",
		"triggers.*.outputs.[].create.relate.from",
		"triggers.*.outputs.[].create.relate.type",
		"triggers.*.outputs.[].create.type",
		// add_to_pile only pushes the triggering entity's ref onto a pile; the
		// pile service caps foreign pushes and the owner's reads stay gated.
		"triggers.*.outputs.[].add_to_pile.create",
		"triggers.*.outputs.[].add_to_pile.owner",
		"triggers.*.outputs.[].add_to_pile.pile",
		// notify pushes the triggering entity's ref onto a declared read-state
		// pile of the people a property names; the recipient reads it gated.
		"triggers.*.outputs.[].notify.pile",
		"triggers.*.outputs.[].notify.reason",
		"triggers.*.outputs.[].notify.to.property",
		"triggers.*.outputs.[].notify.to.type",
		"triggers.*.outputs.[].notify.to.via.[]",
		"triggers.*.outputs.[].check.message",
		"triggers.*.outputs.[].check.severity",
		"triggers.*.outputs.[].check.where",
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
		"app.language",
		"app.world_switcher",

		"styles.*.*",

		"forms.*.title",
		"forms.*.description",
		"forms.*.entity_type",
		"forms.*.mode",
		"forms.*.body",
		"forms.*.fields.[].**",
		"forms.*.relations.[].**",
		"forms.*.steps.[].**",
		// Channel message fields only order and label what realtime.yaml
		// declares; the load check refuses form-only keys.
		"channels.*.messages.*.fields.[].**",
		// A message type's display only folds or hides it in the web app;
		// the messages stay readable everywhere, so it restricts nothing.
		"channels.*.messages.*.display",
		// offer: false only keeps a type out of the web app composer
		// (TKT-WC63SNN); it is not access control, so it restricts nothing.
		"channels.*.messages.*.offer",
		// The cards map of display: cards (TKT-GR1MXL5) only names which
		// listed properties fill a card; the load check validates it
		// against realtime.yaml, and it restricts nothing.
		"channels.*.messages.*.cards.*",
		// A compact session header (TKT-56TPOTE) only folds the state list
		// in the web app; it restricts nothing.
		"channels.*.session.display",
		// A placement (TKT-VXBE16H) only moves the channel type on the
		// entity page; it restricts nothing.
		"channels.*.placement",

		// A busy column (TKT-TU3XH4) is display only, like a kanban busy
		// card field; the save checks it against realtime.yaml.
		"lists.*.columns.[].busy",
		"lists.*.columns.[].direction",
		"lists.*.columns.[].label",
		"lists.*.columns.[].link",
		"lists.*.columns.[].property",
		"lists.*.columns.[].relation",
		"lists.*.columns.[].sortable",
		// Pile toggles only place a button that adds to the caller's own pile.
		"piles.toggles.[].**",
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

		// "Add column" writes through Configure itself (TKT-1X5G3BP).
		"kanbans.*.add_column",
		"kanbans.*.card.**",
		// The heat-map colors of a swimlane board's cells (TKT-83S9G0Z).
		"kanbans.*.cell_colors.**",
		"kanbans.*.column_property",
		// A relation-backed board picks its columns, their order and their
		// colors by naming a relation and properties, like lists.*.group_by.
		"kanbans.*.columns_from.**",
		"kanbans.*.columns.[].**",
		// The summary lines under each column heading (TKT-SVLQQ6).
		"kanbans.*.column_summary.[].**",
		"kanbans.*.condition",
		"kanbans.*.create_form",
		"kanbans.*.edit_form",
		"kanbans.*.entity_type",
		"kanbans.*.filter_controls.[].**",
		// How the board is drawn: columns, or a wall of cards (TKT-0VBQW0H).
		"kanbans.*.layout",
		"kanbans.*.filters.[].**",
		"kanbans.*.footer",
		"kanbans.*.header",
		"kanbans.*.query_scope",
		// Quick-add writes through the ordinary create API (TKT-IP4273S).
		"kanbans.*.quick_add",
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
		"dashboard.cards.[].property",
		"dashboard.cards.[].query",
		"dashboard.cards.[].sort.[].**",
		"dashboard.cards.[].title",

		// A named dashboard has the same editable keys as the default one.
		"dashboards.*.title",
		"dashboards.*.description",
		"dashboards.*.cards.[].columns.[].direction",
		"dashboards.*.cards.[].columns.[].label",
		"dashboards.*.cards.[].columns.[].link",
		"dashboards.*.cards.[].columns.[].property",
		"dashboards.*.cards.[].columns.[].relation",
		"dashboards.*.cards.[].columns.[].sortable",
		"dashboards.*.cards.[].display",
		"dashboards.*.cards.[].group_by",
		"dashboards.*.cards.[].limit",
		"dashboards.*.cards.[].property",
		"dashboards.*.cards.[].query",
		"dashboards.*.cards.[].sort.[].**",
		"dashboards.*.cards.[].title",

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
		"nav.pile",
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
		"version", "namespace", "iri", "includes.**",
		// Abstract types and variants reshape the type set at load, and
		// Configure parses one file without its includes.
		"entities.*.abstract.**", "entities.*.extends",
		// Record types (TKT-J7Q9C4K) decide which rows a write accepts, how
		// long rows are kept and when a ledger period is closed. A retention
		// edit deletes data on the next run, so the section stays on disk.
		"records.**",
		// External commands and upload policy.
		"attachments.**", "transforms.**", "checkers.**",
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
		// A rule that refuses writes (TKT-5CESLHD). Not a secret or a
		// privilege: it is a restriction, so it is changed in the repo, where
		// review sees it. It is protective, and it pins what the rule checks.
		"validations.[].enforce",
		// Access control: a transition guard names an ACL permission.
		"types.*.transitions.[].guard",
		// External sync. A sync connector's Lua finds its records by system,
		// so changing system or sync points reviewed code at other records.
		"entities.*.properties.*.system", "entities.*.properties.*.sync",
		"relations.*.properties.*.system", "relations.*.properties.*.sync",
		// Not in the Configure space yet.
		"copies.**", "conversions.**", "task_item_conversions.**", "default_world", "worlds.**", "entities.*.faces.**",
		"entities.*.properties.*.computed", "relations.*.properties.*.computed",
		"entities.*.properties.*.rollup.**", "relations.*.properties.*.rollup.**",
		"relations.*.scope", "relations.*.owning",
		"validations.[].faces.**",
	},
	FlowsFile: {
		// Stateful flows name permissions, roles and a run_as principal.
		"flows.**",
		// Lua, what a script may reach, and the background job it starts.
		"triggers.*.outputs.[].script.**",
		// A template is a file the project's reviewers wrote.
		"triggers.*.outputs.[].create.template",
		// if_exists: replace deletes the existing target, with no ACL check.
		"triggers.*.outputs.[].create.if_exists",
		// relate.to names a fixed record, which the editor may not be able
		// to read; TriggerChecker refuses it too.
		"triggers.*.outputs.[].relate.**",
		// on.deleted only starts background Lua jobs (TKT-Q0AQ840).
		"triggers.*.on.deleted",
		// Not in the Configure space yet.
		"triggers.*.on.faces.**", "triggers.*.on.copy.**",
	},
	DataEntryFile: {
		"version",
		"app.default_world", "app.disable_custom_injection", "app.max_attachment_bytes", "app.plantuml_server_url",
		// Access control and actions.
		"nav.permission", "nav.action", "spaces.[].permission", "dashboard.cards.[].permission",
		"dashboards.*.cards.[].permission",
		// Page chrome toggles (TKT-31822PN): true, false or {permission}.
		// Not in the Configure space yet; locked with the other permissions.
		"dashboard.validation.**", "dashboards.*.validation.**", "header_buttons.**", "list_buttons.**",
		// Not in the Configure space yet: list-sourced, matrix and ratio cards
		// (TKT-2SGPJZ). The Configure dashboard page edits query cards only.
		"dashboard.cards.[].list", "dashboard.cards.[].rows", "dashboard.cards.[].tone_by",
		"dashboard.cards.[].colors.*", "dashboard.cards.[].match",
		"dashboard.cards.[].rows_label", "dashboard.cards.[].columns_label",
		"dashboard.cards.[].rows_order", "dashboard.cards.[].columns_order",
		"dashboards.*.cards.[].list", "dashboards.*.cards.[].rows", "dashboards.*.cards.[].tone_by",
		"dashboards.*.cards.[].colors.*", "dashboards.*.cards.[].match",
		"dashboards.*.cards.[].rows_label", "dashboards.*.cards.[].columns_label",
		"dashboards.*.cards.[].rows_order", "dashboards.*.cards.[].columns_order",
		// Not in the Configure space yet: records line cards (TKT-RK810V7).
		"dashboard.cards.[].records.**", "dashboards.*.cards.[].records.**",
		// Not in the Configure space yet: card width (TKT-H21TYS). Editing a
		// card in Configure keeps the width it has.
		"dashboard.cards.[].width", "dashboards.*.cards.[].width",
		// Not in the Configure space yet: entity dashboards and their card
		// scopes, which pair with pages: (also locked).
		"dashboard.entity_type", "dashboards.*.entity_type",
		"dashboard.cards.[].scope.*", "dashboards.*.cards.[].scope.*",
		"lists.*.actions.**", "lists.*.toolbar_actions.**", "lists.*.export_render",
		// A board's header actions (TKT-I2SRNWR), like a list's.
		"kanbans.*.actions.**",
		// The pile menu's actions and export transforms, like a list's.
		"piles.actions.[]", "piles.export.[]",
		// Not in the Configure space yet.
		"views.*.export_render",
		"git.**", "palette.**",
		"documents.**", "feeds.**", "caldav.**", "commands.**", "actions.**", "webhooks.**",
		"pages.**", "next_action_bands.**", "next_actions.**", "account.**", "lists.*.create_world",
		// Built-in sidebar rows (TKT-JEAR2UA): not in the Configure space yet.
		"builtins.**", "spaces.[].builtins.**",
		// Where suggestions appear (TKT-6EEXXE), locked like next_actions.
		// Calendars and gantts carry the key too, but those trees are
		// editable whole (calendars.**, gantts.**).
		"next_action_surfaces.*", "dashboard.suggestions", "dashboards.*.suggestions",
		"lists.*.suggestions", "kanbans.*.suggestions", "views.*.suggestions",
		"lists.*.columns.[].face", "dashboard.cards.[].columns.[].face",
		"dashboards.*.cards.[].columns.[].face",
		// Busy columns are list-only (TKT-TU3XH4); the load refuses them here.
		"dashboard.cards.[].columns.[].busy", "dashboards.*.cards.[].columns.[].busy",
		// Editable columns are view-table-only (TKT-4Q2EIVZ); the load refuses them here.
		"lists.*.columns.[].render", "dashboard.cards.[].columns.[].render",
		"dashboards.*.cards.[].columns.[].render",
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
		"validations.[].enforce",
		// A record type's retention and a ledger's balance and lock rules
		// limit what is kept and what may be posted.
		"records.*.retention",
		"records.*.ledger.**",
	},
	DataEntryFile: {
		"app.disable_custom_injection", "app.max_attachment_bytes",
		"nav.permission", "spaces.[].permission", "dashboard.cards.[].permission",
		"dashboards.*.cards.[].permission",
		"dashboard.validation.**", "dashboards.*.validation.**", "header_buttons.**", "list_buttons.**",
	},
	// A script may stamp what acl.yaml decides access by; removing it
	// would drop that stamp from every record written afterwards.
	FlowsFile: {"triggers.*.outputs.[].script.**"},
}

// A pin keeps the editable keys of an item that say what its locked keys
// apply to from changing while the item holds one of them.
type pin struct {
	item string   // pattern of the item
	keys []string // its keys that cannot change
	by   []string // patterns of the locked keys that pin them
	// order lists keys of the item holding a list whose items stay as they
	// were read: edited in place, never added, removed or moved, because
	// the order is what runs first.
	order []string
}

// ordered lists the mappings whose key order is behavior: flows.yaml
// triggers fire in document order, so moving one changes what a later
// trigger, or a script, sees. The kept keys of such a mapping keep their
// relative order, and a new key goes after all of them, so a new trigger
// never runs before one that was there. The browser has no control that
// reorders them.
var ordered = map[File][]string{
	FlowsFile: {"triggers"},
}

// pinned lists the pins. Moving a guarded transition elsewhere would free
// the edge it guarded, and adding that edge back without a guard is an
// ordinary edit. (A second edge with the same from and to is refused by the
// state machine itself.) Pointing reviewed Lua, or an action that may reach
// past the ACL, at other records is a new use of that code, so the trigger
// and scope of an item holding it stay as they are. A rule enforced on write
// is a restriction: what it applies to and what it requires stay as reviewed.
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
			item: "validations.[]",
			keys: []string{"entity_type", "faces", "when", "when_condition", "then", "then_condition"},
			by:   []string{"validations.[].enforce"},
		},
	},
	FlowsFile: {
		// A trigger holding a script keeps what fires it, and the place of
		// the script among its outputs.
		{
			item: "triggers.*", keys: []string{"on"}, order: []string{"outputs"},
			by: []string{"triggers.*.outputs.[].script.**"},
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
	// Change is "added", "changed", "removed" or "reordered".
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
		for _, k := range p.order {
			old, _ := before.Get(k)
			now, _ := after.Get(k)
			was, _ := old.([]any)
			items, _ := now.([]any)
			switch {
			case !originsInOrder(items):
				c.refuse(with(display, k), "reordered")
			case !originsIntact(items, len(was)):
				c.refuse(with(display, k), "changed")
			}
		}
	}
}

// originsInOrder reports whether the items read from the file are still in
// the order they were read in. New items have no origin and are skipped.
func originsInOrder(items []any) bool {
	last := -1
	for _, item := range items {
		m, ok := item.(*Map)
		if !ok || !m.HasOrigin {
			continue
		}
		if m.Origin < last {
			return false
		}
		last = m.Origin
	}
	return true
}

// originsIntact reports whether items are exactly the n items read from
// the file, in the order they were read in.
func originsIntact(items []any, n int) bool {
	if len(items) != n {
		return false
	}
	for i, item := range items {
		if m, ok := item.(*Map); !ok || !m.HasOrigin || m.Origin != i {
			return false
		}
	}
	return true
}

// newKeysLast reports whether every key after adds to before comes after
// all the keys it keeps.
func newKeysLast(before, after *Map) bool {
	added := false
	for _, k := range after.Keys {
		if _, ok := before.Get(k); !ok {
			added = true
		} else if added {
			return false
		}
	}
	return true
}

// keysInOrder reports whether the keys after keeps from before are in the
// same relative order.
func keysInOrder(before, after *Map) bool {
	pos := make(map[string]int, len(before.Keys))
	for i, k := range before.Keys {
		pos[k] = i
	}
	last := -1
	for _, k := range after.Keys {
		i, ok := pos[k]
		if !ok {
			continue
		}
		if i < last {
			return false
		}
		last = i
	}
	return true
}

// diff walks both trees together. path is the pattern form (list items as
// "[]"); display is the same path with list indexes, for messages.
func (c *checker) diff(path, display []string, before, after any) {
	bm, bIsMap := before.(*Map)
	am, aIsMap := after.(*Map)
	if bIsMap && aIsMap {
		if anyMatch(ordered[c.file], normalize(c.file, path), match) &&
			(!keysInOrder(bm, am) || !newKeysLast(bm, am)) {

			c.refuse(display, "reordered")
		}
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
	if !keysInOrder(bm, am) {
		// Key order is behavior in places (trigger firing order), and
		// layout everywhere else; either way the record says it moved.
		add([]string{strings.TrimPrefix(strings.Join(path, ".")+" (order)", " ")})
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
