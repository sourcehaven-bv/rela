package docs

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// luaRolesMatrix emits a role × verb capability table for an entity type (or,
// with no type, for every type). Rows are entity types (from the metamodel),
// columns are roles (from the policy). A ✓ means the role grants that verb on
// that type. roles_matrix{type="risico"} or roles_matrix{}.
func (a *aclBindings) luaRolesMatrix(ls *lua.LState) int {
	if a.policy == nil {
		a.emit("_No access policy defined (no `acl.yaml`)._\n\n")
		return 0
	}
	tbl := argTable(ls)
	typ := fieldString(ls, tbl, "type")

	types := a.matrixTypes(typ)
	if len(types) == 0 {
		return a.luaFail(ls, "roles_matrix: unknown entity type %q", typ)
	}
	// The built-in "everyone" role is not a column: acl appends it to EVERY
	// principal's effective role set, so its grants must be folded into every
	// named role's cell (else the table understates effective access — the whole
	// point of the matrix). It is reported once, in a footnote.
	everyone, hasEveryone := a.policy.Roles[acl.EveryoneRole]
	roles := namedRoleNames(a.policy.Roles)
	if len(roles) == 0 && !hasEveryone {
		a.emit("_No roles defined in the policy._\n\n")
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
	facedRead := false
	for _, t := range types {
		for _, v := range verbs {
			fmt.Fprintf(&b, "| `%s` | %s |", mdCell(t), v.name)
			everyoneGrant := hasEveryone && roleGrantsVerb(everyone, v, t)
			if everyoneGrant {
				everyoneGrantsAny = true
			}
			for _, rn := range roles {
				// Effective grant = the role's own grant OR everyone's.
				if roleGrantsVerb(a.policy.Roles[rn], v, t) || everyoneGrant {
					suffix := readScopeSuffix(a.policy.Roles[rn], everyone, hasEveryone, v, t)
					if suffix != "" {
						facedRead = true
					}
					fmt.Fprintf(&b, " ✓%s |", suffix)
				} else {
					b.WriteString("  |")
				}
			}
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	if facedRead {
		b.WriteString("_A `✓ @face` read cell is scoped to that face: the role reads only " +
			"that version of the entity, not every one._\n\n")
	}
	if everyoneGrantsAny {
		b.WriteString("_✓ cells include grants from the built-in `everyone` role " +
			"(applies to all principals, incl. unauthenticated)._\n\n")
	}
	a.emit(b.String())
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
		return acl.GrantsRead(role, typ)
	}
	return acl.GrantsVerb(role, v.op, typ)
}

// matrixTypes returns the requested type (validated) or all entity types.
func (a *aclBindings) matrixTypes(typ string) []string {
	if typ != "" {
		if _, ok := a.meta.GetEntityDef(typ); !ok {
			return nil
		}
		return []string{typ}
	}
	all := a.meta.EntityTypes()
	sort.Strings(all)
	return all
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

// readScopeSuffix qualifies a read ✓ with the faces the grant is limited to.
//
// A bare ✓ overstates a face-scoped grant. `read: [policy@published]` and
// `read: [policy]` both make [acl.GrantsRead] true — deliberately, since a face
// grant does grant its type for the purposes of composing a query — so a table
// built on that predicate alone renders a reader who sees only published
// policies identically to an editor who sees every draft. That is the whole
// distinction a face-scoped grant exists to draw, rendered invisible.
//
// Returns "" for a full-face grant, so the common case stays a plain ✓ and only
// a genuinely narrowed grant carries the annotation.
func readScopeSuffix(role, everyone acl.RoleDef, hasEveryone bool, v verbSpec, typ string) string {
	if v.name != "read" {
		return ""
	}
	faces, all := acl.ReadFaces(role, typ)
	if all || len(faces) == 0 {
		return ""
	}
	// The everyone role widens: its grants are folded into every cell, so a
	// role narrowed to one face still reads every face if `everyone` grants the
	// type outright. Annotating it would understate the effective access, which
	// is the direction that matters for a security document.
	if hasEveryone && acl.GrantsRead(everyone, typ) {
		if _, everyoneAll := acl.ReadFaces(everyone, typ); everyoneAll {
			return ""
		}
	}
	names := make([]string, 0, len(faces))
	for _, f := range faces {
		names = append(names, f.String())
	}
	sort.Strings(names)
	return " @" + strings.Join(names, "/")
}

// luaWorldsMatrix emits a role × world table: which worlds each role may ask
// for. worlds_matrix{}.
//
// # Why this is not a column in roles_matrix{}
//
// A world is a NAVIGATION fact, not an authorization one. `published` and
// `editorial` hold the same entities; a world says which view of them a client
// may request, not what it may do to any of them. Folding it into the per-type
// verb table would imply worlds are a fifth verb alongside create/read/update/
// delete, and would repeat one per-role row under every entity type.
//
// # The default world is the absence of a grant
//
// acl.RoleDef.Worlds is empty for a role that names no `world:` entry, and that
// means the default world ONLY — never "no worlds". Rendering an empty list as
// an empty row would read as "this role may ask for nothing", the exact inverse
// of the truth, so the default world is a column every role ticks.
func (a *aclBindings) luaWorldsMatrix(ls *lua.LState) int {
	if a.policy == nil {
		a.emit("_No access policy defined (no `acl.yaml`)._\n\n")
		return 0
	}
	if tbl := argTable(ls); tbl != nil && rejectUnknownKeys(a, ls, "worlds_matrix", tbl) {
		return 0
	}
	// A project declaring no worlds has one view of the graph, so a table whose
	// only column is the default world states nothing. Refusing is the same call
	// shows{} makes for a claim that asserts nothing.
	declared := declaredWorldNames(a.meta)
	if len(declared) == 0 {
		return a.luaFail(ls, "worlds_matrix: this project declares no worlds, so every role "+
			"sees the same single view and the table would assert nothing. Remove the call, "+
			"or document a project with `worlds:` in schema.yaml")
	}

	everyone, hasEveryone := a.policy.Roles[acl.EveryoneRole]
	roles := namedRoleNames(a.policy.Roles)
	if len(roles) == 0 && !hasEveryone {
		a.emit("_No roles defined in the policy._\n\n")
		return 0
	}

	// The default world leads: it is what a client gets by asking for nothing,
	// so it is the column a reader should read first.
	columns := append([]string{metamodel.DefaultWorldName}, declared...)

	var b strings.Builder
	b.WriteString("| Role |")
	for _, w := range columns {
		fmt.Fprintf(&b, " `%s` |", w)
	}
	b.WriteString("\n|---|")
	for range columns {
		b.WriteString("---|")
	}
	b.WriteString("\n")

	for _, rn := range roles {
		fmt.Fprintf(&b, "| %s |", mdCell(rn))
		grants := worldSet(a.policy.Roles[rn])
		if hasEveryone {
			for w := range worldSet(everyone) {
				grants[w] = struct{}{}
			}
		}
		for _, w := range columns {
			if _, ok := grants[w]; ok {
				b.WriteString(" ✓ |")
			} else {
				b.WriteString("  |")
			}
		}
		b.WriteString("\n")
	}
	b.WriteString("\n_Every role may ask for the default world; it is the view served when a " +
		"request names none. A world a role may not ask for answers exactly as an empty " +
		"one does._\n\n")

	a.emit(b.String())
	return 0
}

// worldSet is the set of worlds a role may ask for, always including the
// default world (which is never an explicit grant — see acl.RoleDef.Worlds).
//
// It reads BOTH the split Worlds list and any surviving `world:` token in Read.
// acl.Policy.Validate moves those tokens from Read into Worlds, and every
// loaded policy has passed it — but a Policy built in memory has not, and there
// the split has not happened. Reading only Worlds would then render every role
// as holding no world but the default: a table that silently understates access
// rather than failing, which is the worst of the three outcomes.
func worldSet(role acl.RoleDef) map[string]struct{} {
	out := map[string]struct{}{metamodel.DefaultWorldName: {}}
	for _, w := range role.Worlds {
		out[w] = struct{}{}
	}
	for _, r := range role.Read {
		if name, ok := strings.CutPrefix(r, acl.WorldGrantPrefix); ok && name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

// declaredWorldNames lists the schema's declared world names, sorted.
func declaredWorldNames(m *metamodel.Metamodel) []string {
	out := make([]string, 0, len(m.Worlds))
	for name := range m.Worlds {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
