package docs

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// luaRolesMatrix emits a role × verb capability table for an entity type (or,
// with no type, for every type). Rows are entity types (from the metamodel),
// columns are roles (from the policy). A ✓ means the role grants that verb on
// that type. roles_matrix{type="risico"} or roles_matrix{}.
func (dr *docRuntime) luaRolesMatrix(ls *lua.LState) int {
	if dr.policy == nil {
		dr.emit("_No access policy defined (no `acl.yaml`)._\n\n")
		return 0
	}
	tbl := argTable(ls)
	typ := fieldString(ls, tbl, "type")

	types := dr.matrixTypes(typ)
	if len(types) == 0 {
		return dr.luaFail(ls, "roles_matrix: unknown entity type %q", typ)
	}
	roles := sortedRoleNames(dr.policy.Roles)
	if len(roles) == 0 {
		dr.emit("_No roles defined in the policy._\n\n")
		return 0
	}

	var b strings.Builder
	// Header: Type | Verb | role1 | role2 ...
	b.WriteString("| Type | Verb |")
	for _, r := range roles {
		fmt.Fprintf(&b, " %s |", r)
	}
	b.WriteString("\n|---|---|")
	for range roles {
		b.WriteString("---|")
	}
	b.WriteString("\n")

	verbs := []struct {
		name string
		op   acl.Op
	}{
		{"create", acl.OpCreate},
		{"read", ""}, // read handled specially
		{"update", acl.OpUpdate},
		{"delete", acl.OpDelete},
	}
	for _, t := range types {
		for _, v := range verbs {
			fmt.Fprintf(&b, "| `%s` | %s |", t, v.name)
			for _, rn := range roles {
				role := dr.policy.Roles[rn]
				var granted bool
				if v.name == "read" {
					granted = grantsList(role.Read, t)
				} else {
					granted = grantsVerb(role, v.op, t)
				}
				if granted {
					b.WriteString(" ✓ |")
				} else {
					b.WriteString("  |")
				}
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	dr.emit(b.String())
	return 0
}

// matrixTypes returns the requested type (validated) or all entity types.
func (dr *docRuntime) matrixTypes(typ string) []string {
	if typ != "" {
		if _, ok := dr.meta.GetEntityDef(typ); !ok {
			return nil
		}
		return []string{typ}
	}
	all := dr.meta.EntityTypes()
	sort.Strings(all)
	return all
}

// grantsVerb / grantsList replicate the policy's wildcard-or-exact match over
// the exported RoleDef grant lists (the policy's own helpers are unexported).
func grantsVerb(role acl.RoleDef, op acl.Op, target string) bool {
	switch op {
	case acl.OpCreate:
		return grantsList(role.Create, target)
	case acl.OpUpdate, acl.OpRename:
		return grantsList(role.Update, target)
	case acl.OpDelete:
		return grantsList(role.Delete, target)
	}
	return false
}

func grantsList(list []string, target string) bool {
	for _, t := range list {
		if t == "*" || t == target {
			return true
		}
	}
	return false
}

func sortedRoleNames(roles map[string]acl.RoleDef) []string {
	names := make([]string, 0, len(roles))
	for n := range roles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
