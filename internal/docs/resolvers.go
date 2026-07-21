package docs

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// entityDef looks up an entity type, raising a fail-loud resolve error naming
// the type when unknown.
func (dr *docRuntime) entityDef(ls *lua.LState, fn, typ string) (*metamodel.EntityDef, bool) {
	if typ == "" {
		dr.luaFail(ls, "resolve", "%s: `type` is required", fn)
		return nil, false
	}
	def, ok := dr.meta.GetEntityDef(typ)
	if !ok {
		dr.luaFail(ls, "resolve", "%s: unknown entity type %q", fn, typ)
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
		return dr.luaFail(ls, "resolve", "values: %q has no field %q", typ, field)
	}

	values, labels, descs, def2 := dr.resolveEnum(prop)
	if len(values) == 0 {
		return dr.luaFail(ls, "resolve", "values: field %q of %q is not an enum", field, typ)
	}

	var b strings.Builder
	hasDesc := len(descs) > 0
	if hasDesc {
		b.WriteString("| Value | Meaning |\n|---|---|\n")
		for _, v := range values {
			shown := v
			if l := labels[v]; l != "" {
				shown = fmt.Sprintf("%s (`%s`)", l, v)
			} else {
				shown = fmt.Sprintf("`%s`", v)
			}
			if v == def2 {
				shown += " _(default)_"
			}
			fmt.Fprintf(&b, "| %s | %s |\n", shown, mdCell(descs[v]))
		}
	} else {
		for i, v := range values {
			if i > 0 {
				b.WriteString(" · ")
			}
			fmt.Fprintf(&b, "`%s`", v)
			if v == def2 {
				b.WriteString(" (default)")
			}
		}
	}
	b.WriteString("\n\n")
	dr.emit(b.String())
	return 0
}

// resolveEnum returns the value set for a property: a named custom type's
// values/labels/descriptions/default, else an inline enum's values.
func (dr *docRuntime) resolveEnum(prop metamodel.PropertyDef) (values []string, labels, descs map[string]string, def string) {
	if ct, named := dr.meta.Types[prop.Type]; named {
		return ct.Values, ct.Labels, ct.Descriptions, ct.Default
	}
	return prop.Values, prop.Labels, nil, prop.Default
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
		if !containsStr(rel.From, typ) {
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
		return dr.luaFail(ls, "resolve", "entity: `id` is required")
	}
	e, err := dr.store.GetEntity(dr.ctx, id)
	if err != nil {
		return dr.luaFail(ls, "resolve", "entity: %q not found in the seeded graph (did you create() it?)", id)
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

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

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

// countType is a small shared helper: number of seeded entities of a type.
func (dr *docRuntime) countType(typ string) (int, error) {
	n := 0
	for _, err := range dr.store.ListEntities(dr.ctx, store.EntityQuery{Type: typ}) {
		if err != nil {
			return 0, err
		}
		n++
	}
	return n, nil
}
