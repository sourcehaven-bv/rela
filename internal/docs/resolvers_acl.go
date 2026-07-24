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
	// The built-in "everyone" role is not a column: acl appends it to EVERY
	// principal's effective role set, so its grants must be folded into every
	// named role's cell (else the table understates effective access — the whole
	// point of the matrix). It is reported once, in a footnote.
	everyone, hasEveryone := dr.policy.Roles[acl.EveryoneRole]
	roles := namedRoleNames(dr.policy.Roles)
	if len(roles) == 0 && !hasEveryone {
		dr.emit("_No roles defined in the policy._\n\n")
		return 0
	}

	var b strings.Builder
	b.WriteString("| Type | Verb |")
	for _, r := range roles {
		fmt.Fprintf(&b, " %s |", r)
	}
	b.WriteString("\n|---|---|")
	for range roles {
		b.WriteString("---|")
	}
	b.WriteString("\n")

	verbs := []verbSpec{
		{"create", acl.OpCreate},
		{"read", ""}, // read handled specially
		{"update", acl.OpUpdate},
		{"delete", acl.OpDelete},
	}
	everyoneGrantsAny := false
	for _, t := range types {
		for _, v := range verbs {
			fmt.Fprintf(&b, "| `%s` | %s |", mdCell(t), v.name)
			everyoneGrant := hasEveryone && roleGrantsVerb(everyone, v, t)
			if everyoneGrant {
				everyoneGrantsAny = true
			}
			for _, rn := range roles {
				// Effective grant = the role's own grant OR everyone's.
				if roleGrantsVerb(dr.policy.Roles[rn], v, t) || everyoneGrant {
					b.WriteString(" ✓ |")
				} else {
					b.WriteString("  |")
				}
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	if everyoneGrantsAny {
		b.WriteString("_✓ cells include grants from the built-in `everyone` role " +
			"(applies to all principals, incl. unauthenticated)._\n\n")
	}
	dr.emit(b.String())
	return 0
}

// verbSpec pairs a matrix column verb name with its acl.Op (read has none — it
// is a separate grant list).
type verbSpec struct {
	name string
	op   acl.Op
}

// roleGrantsVerb reports whether a role grants the given verb on a type,
// dispatching read (a separate list) from the create/update/delete verbs.
func roleGrantsVerb(role acl.RoleDef, v verbSpec, typ string) bool {
	if v.name == "read" {
		return grantsList(role.Read, typ)
	}
	return grantsVerb(role, v.op, typ)
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

// namedRoleNames returns the policy's role names sorted, EXCLUDING the built-in
// "everyone" role (which is folded into every column rather than shown as one).
func namedRoleNames(roles map[string]acl.RoleDef) []string {
	names := make([]string, 0, len(roles))
	for n := range roles {
		if n == acl.EveryoneRole {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
