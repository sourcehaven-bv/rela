package lua

import (
	"context"
	"errors"
	"slices"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// errRoleHoldersUnavailable is raised by rela.principals_with_role on a
// runtime built without the role-holder capability.
var errRoleHoldersUnavailable = errors.New("role holders are not available in this context")

// RoleHolders answers which user entities hold a rela role through the IdP
// roles the webhook stored (TKT-IZL6NX8). Wiring supplies it; this package
// filters the answer through [ReadDeps.VisibleReader].
type RoleHolders interface {
	// PrincipalsWithRole returns the user entity type and the rows of the
	// user entities holding role, ungated. An undeclared role is an error.
	PrincipalsWithRole(ctx context.Context, role string) (entityType string, refs []entity.Ref, err error)
}

// luaPrincipalsWithRole implements rela.principals_with_role(role) -> a
// sorted list of user entity ids. Only ids the script can read are returned:
// one batched header read through the script's gate settles all of them,
// whatever their number.
func (r *Runtime) luaPrincipalsWithRole(ls *lua.LState) int {
	role := ls.CheckString(1)
	if r.deps.RoleHolders == nil {
		ls.RaiseError("rela.principals_with_role: %s", errRoleHoldersUnavailable)
		return 0
	}
	rd, ok := r.reader(ls, "rela.principals_with_role")
	if !ok {
		return 0
	}
	hr, ok := rd.(headerResolver)
	if !ok {
		ls.RaiseError("rela.principals_with_role: the reader cannot resolve entities")
		return 0
	}
	ctx := r.callerCtx()
	userType, refs, err := r.deps.RoleHolders.PrincipalsWithRole(ctx, role)
	if err != nil {
		ls.RaiseError("rela.principals_with_role: %s", err.Error())
		return 0
	}
	resolved, err := hr.ResolveHeadersErr(ctx, refs)
	if err != nil {
		ls.RaiseError("rela.principals_with_role: %s", err.Error())
		return 0
	}
	var readable []string
	for _, ref := range refs {
		if h, ok := resolved[ref]; ok && h.Served() && h.Header.Type == userType {
			readable = append(readable, ref.ID)
		}
	}
	slices.Sort(readable)
	readable = slices.Compact(readable)
	out := ls.NewTable()
	for _, id := range readable {
		out.Append(lua.LString(id))
	}
	ls.Push(out)
	return 1
}
