// Lua bindings for schema introspection (rela.get_entity_types,
// rela.get_relation_types) and entity sorting (rela.sort_entities).
package lua

import (
	"math"
	"sort"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// schemaBindings implements the schema-introspection bindings. A type of its
// own rather than more methods on [Runtime] (the urlHelpers rationale in
// urls.go): the bindings need exactly one thing from the runtime — the loaded
// metamodel — so they hold that and nothing else. Registration stays on
// Runtime, in registerReadBindings.
type schemaBindings struct {
	meta *metamodel.Metamodel
}

// luaGetEntityTypes implements rela.get_entity_types() -> table
// Returns a table of entity type definitions with their properties.
func (b schemaBindings) luaGetEntityTypes(ls *lua.LState) int {
	result := ls.NewTable()

	for name, et := range b.meta.Entities {
		typeTable := ls.NewTable()
		typeTable.RawSetString("name", lua.LString(name))
		typeTable.RawSetString("label", lua.LString(et.Label))
		typeTable.RawSetString("plural", lua.LString(et.Plural))

		// Properties
		propsTable := ls.NewTable()
		for propName, prop := range et.Properties {
			propTable := ls.NewTable()
			propTable.RawSetString("name", lua.LString(propName))
			propTable.RawSetString("type", lua.LString(prop.Type))
			propTable.RawSetString("required", lua.LBool(prop.Required))
			if prop.Default != "" {
				propTable.RawSetString("default", lua.LString(prop.Default))
			}
			if len(prop.Values) > 0 {
				valuesTable := ls.NewTable()
				for i, val := range prop.Values {
					valuesTable.RawSetInt(i+1, lua.LString(val))
				}
				propTable.RawSetString("values", valuesTable)
			}
			propsTable.RawSetString(propName, propTable)
		}
		typeTable.RawSetString("properties", propsTable)

		result.RawSetString(name, typeTable)
	}

	ls.Push(result)
	return 1
}

// luaGetRelationTypes implements rela.get_relation_types() -> table
// Returns a table of relation type definitions with their constraints.
func (b schemaBindings) luaGetRelationTypes(ls *lua.LState) int {
	result := ls.NewTable()

	for name, rt := range b.meta.Relations {
		typeTable := ls.NewTable()
		typeTable.RawSetString("name", lua.LString(name))
		typeTable.RawSetString("label", lua.LString(rt.Label))

		// From constraints
		fromTable := ls.NewTable()
		for i, f := range rt.From {
			fromTable.RawSetInt(i+1, lua.LString(f))
		}
		typeTable.RawSetString("from", fromTable)

		// To constraints
		toTable := ls.NewTable()
		for i, t := range rt.To {
			toTable.RawSetInt(i+1, lua.LString(t))
		}
		typeTable.RawSetString("to", toTable)

		result.RawSetString(name, typeTable)
	}

	ls.Push(result)
	return 1
}

// sortableEntry holds an entity table and its sort key for sorting.
type sortableEntry struct {
	value lua.LValue
	prop  lua.LValue
}

// luaSortEntities implements rela.sort_entities(entities, property, direction?) -> table
// Sorts a list of entity tables by a property value.
// Direction is optional: "asc" (default) or "desc".
// Handles numeric comparison for property values that look like numbers.
//
// A plain function rather than a *Runtime method: it operates purely on its
// Lua arguments and touches no runtime state.
func luaSortEntities(ls *lua.LState) int {
	entitiesTable := ls.CheckTable(1)
	property := ls.CheckString(2)
	direction := ls.OptString(3, "asc")

	if property == "" {
		ls.RaiseError("sort_entities: property cannot be empty")
		return 0
	}

	descending := direction == "desc"

	// Collect entities into a slice for sorting
	entries := make([]sortableEntry, 0, entitiesTable.Len())

	for i := 1; i <= entitiesTable.Len(); i++ {
		v := entitiesTable.RawGetInt(i)
		tbl, ok := v.(*lua.LTable)
		if !ok {
			continue
		}
		props := tbl.RawGetString("properties")
		propVal := lua.LNil
		if propsTbl, ok := props.(*lua.LTable); ok {
			propVal = propsTbl.RawGetString(property)
		}
		entries = append(entries, sortableEntry{value: v, prop: propVal})
	}

	// Sort entries using bubble sort (sufficient for typical entity counts)
	sortEntries(entries, descending)

	// Build result table
	result := ls.NewTable()
	for i, entry := range entries {
		result.RawSetInt(i+1, entry.value)
	}

	ls.Push(result)
	return 1
}

// sortEntries sorts entity entries by their property value. Stable so
// entries with equal sort keys keep their input order.
func sortEntries(entries []sortableEntry, descending bool) {
	sort.SliceStable(entries, func(i, j int) bool {
		if descending {
			return entryLess(entries[j].prop, entries[i].prop)
		}
		return entryLess(entries[i].prop, entries[j].prop)
	})
}

// entryLess reports whether a sorts before b. Two numeric values compare
// numerically; otherwise both compare as strings.
func entryLess(a, b lua.LValue) bool {
	aStr, aNum, aIsNum := luaValueToSortable(a)
	bStr, bNum, bIsNum := luaValueToSortable(b)
	if aIsNum && bIsNum {
		return aNum < bNum
	}
	return aStr < bStr
}

// luaValueToSortable converts a Lua value to sortable string and number
// representations. A string is treated as numeric only when it parses
// *entirely* as a number — strconv.ParseFloat over the trimmed value —
// so "1.2.0" or "3 blind mice" sort lexicographically rather than being
// silently reduced to their numeric prefix (1 and 3), which the old
// fmt.Sscanf("%f") accepted.
func luaValueToSortable(v lua.LValue) (str string, num float64, isNum bool) {
	switch val := v.(type) {
	case lua.LNumber:
		return "", float64(val), true
	case lua.LString:
		s := string(val)
		if n, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return s, n, true
		}
		return s, 0, false
	case *lua.LNilType:
		return "", math.MaxFloat64, false // nil sorts last
	default:
		return v.String(), 0, false
	}
}
