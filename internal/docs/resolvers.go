package docs

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// entityDef looks up an entity type, raising a fail-loud resolve error naming
// the type when unknown.
func (dr *docRuntime) entityDef(ls *lua.LState, fn, typ string) (*metamodel.EntityDef, bool) {
	if typ == "" {
		dr.luaFail(ls, "%s: `type` is required", fn)
		return nil, false
	}
	def, ok := dr.meta.GetEntityDef(typ)
	if !ok {
		dr.luaFail(ls, "%s: unknown entity type %q", fn, typ)
		return nil, false
	}
	return def, true
}

// propertyNames returns an entity type's properties in definition order, or
// alphabetical when no order was captured.
func propertyNames(def *metamodel.EntityDef) []string {
	if order := def.GetPropertyOrder(); len(order) > 0 {
		return order
	}
	names := make([]string, 0, len(def.Properties))
	for n := range def.Properties {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// luaTyperef emits a Markdown table of an entity type's fields.
// typeref{type="risico", fields="required"|"all"}.
func (dr *docRuntime) luaTyperef(ls *lua.LState) int {
	tbl := argTable(ls)
	typ := fieldString(ls, tbl, "type")
	which := fieldString(ls, tbl, "fields")
	def, ok := dr.entityDef(ls, "typeref", typ)
	if !ok {
		return 0
	}
	requiredOnly := which == "required"

	var b strings.Builder
	b.WriteString("| Field | Type | Required |\n|---|---|---|\n")
	for _, name := range propertyNames(def) {
		p := def.Properties[name]
		if requiredOnly && !p.Required {
			continue
		}
		typeName := p.Type
		if typeName == "" {
			typeName = "string"
		}
		req := ""
		if p.Required {
			req = "yes"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s |\n", name, typeName, req)
	}
	b.WriteString("\n")
	dr.emit(b.String())
	return 0
}

// enumInfo is a property's resolved value set for the values resolver.
type enumInfo struct {
	values       []string
	labels       map[string]string
	descriptions map[string]string
	def          string
}

// luaValues emits an enum field's allowed values, marking the default and
// adding per-value meaning when CustomType.Descriptions is present.
// values{type="risico", field="behandeling"}.
func (dr *docRuntime) luaValues(ls *lua.LState) int {
	tbl := argTable(ls)
	typ := fieldString(ls, tbl, "type")
	field := fieldString(ls, tbl, "field")
	def, ok := dr.entityDef(ls, "values", typ)
	if !ok {
		return 0
	}
	prop, ok := def.Properties[field]
	if !ok {
		return dr.luaFail(ls, "values: %q has no field %q", typ, field)
	}

	e := dr.resolveEnum(prop)
	if len(e.values) == 0 {
		return dr.luaFail(ls, "values: field %q of %q is not an enum", field, typ)
	}

	if len(e.descriptions) > 0 {
		dr.emit(e.renderTable())
	} else {
		dr.emit(e.renderList())
	}
	return 0
}

// renderTable renders enum values as a Value|Meaning table (used when the type
// carries per-value descriptions).
func (e enumInfo) renderTable() string {
	var b strings.Builder
	b.WriteString("| Value | Meaning |\n|---|---|\n")
	for _, v := range e.values {
		shown := fmt.Sprintf("`%s`", v)
		if l := e.labels[v]; l != "" {
			shown = fmt.Sprintf("%s (`%s`)", l, v)
		}
		if v == e.def {
			shown += " _(default)_"
		}
		fmt.Fprintf(&b, "| %s | %s |\n", shown, mdCell(e.descriptions[v]))
	}
	b.WriteString("\n")
	return b.String()
}

// renderList renders enum values as a compact inline list (no descriptions).
func (e enumInfo) renderList() string {
	var b strings.Builder
	for i, v := range e.values {
		if i > 0 {
			b.WriteString(" · ")
		}
		fmt.Fprintf(&b, "`%s`", v)
		if v == e.def {
			b.WriteString(" (default)")
		}
	}
	b.WriteString("\n\n")
	return b.String()
}

// resolveEnum returns the value set for a property: a named custom type's
// values/labels/descriptions/default, else an inline enum's values.
func (dr *docRuntime) resolveEnum(prop metamodel.PropertyDef) enumInfo {
	if ct, named := dr.meta.Types[prop.Type]; named {
		return enumInfo{ct.Values, ct.Labels, ct.Descriptions, ct.Default}
	}
	return enumInfo{prop.Values, prop.Labels, nil, prop.Default}
}

// luaRelations emits an entity type's outgoing relations as a list.
// relations{type="bedrijfsmiddel"}.
func (dr *docRuntime) luaRelations(ls *lua.LState) int {
	tbl := argTable(ls)
	typ := fieldString(ls, tbl, "type")
	if _, ok := dr.entityDef(ls, "relations", typ); !ok {
		return 0
	}

	names := make([]string, 0, len(dr.meta.Relations))
	for name := range dr.meta.Relations {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		rel := dr.meta.Relations[name]
		if !slices.Contains(rel.From, typ) {
			continue
		}
		fmt.Fprintf(&b, "- `%s` → %s", name, strings.Join(rel.To, ", "))
		if rel.Description != "" {
			fmt.Fprintf(&b, " — %s", oneLine(rel.Description))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
	dr.emit(b.String())
	return 0
}

// luaEntity emits one seeded entity's fields as a definition list.
// entity{id="risico-1", fields={"kans","impact"}}.
func (dr *docRuntime) luaEntity(ls *lua.LState) int {
	tbl := argTable(ls)
	id := fieldString(ls, tbl, "id")
	if id == "" {
		id = argString(ls, "id")
	}
	if id == "" {
		return dr.luaFail(ls, "entity: `id` is required")
	}
	e, err := dr.store.GetEntity(dr.ctx, id)
	if err != nil {
		return dr.luaFail(ls, "entity: %q not found in the seeded graph (did you create() it?)", id)
	}
	wanted := fieldStringSlice(ls, tbl, "fields")

	var b strings.Builder
	fmt.Fprintf(&b, "**%s**\n\n", id)
	emitField := func(name string) {
		if v, ok := e.Properties[name]; ok {
			fmt.Fprintf(&b, "- %s: %v\n", name, v)
		}
	}
	if len(wanted) > 0 {
		for _, name := range wanted {
			emitField(name)
		}
	} else {
		keys := make([]string, 0, len(e.Properties))
		for k := range e.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			emitField(k)
		}
	}
	b.WriteString("\n")
	dr.emit(b.String())
	return 0
}

// luaDescription returns the deployment's top-level description (metamodel), or
// empty (subject to strict mode at the echo site). description().
func (dr *docRuntime) luaDescription(ls *lua.LState) int {
	ls.Push(lua.LString(dr.meta.Description))
	return 1
}

// --- small text helpers ---

// oneLine collapses newlines so a description fits on a list line.
func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

// mdCell makes a string safe inside a Markdown table cell (flatten newlines,
// escape pipes).
func mdCell(s string) string {
	s = oneLine(s)
	return strings.ReplaceAll(s, "|", "\\|")
}
