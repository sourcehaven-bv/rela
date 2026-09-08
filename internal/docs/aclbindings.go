package docs

import (
	"context"

	lua "github.com/yuin/gopher-lua"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// aclBindings owns the doc.* verbs that answer questions about the ACL policy:
// the descriptive tables (roles_matrix{}, worlds_matrix{}) and the write-side
// assertions (refuses{}, permits{}).
//
// # Why this is a cluster
//
// These are the only verbs that read [acl.Policy], and grouping them keeps the
// policy off the verbs that have no business seeing it — the same containment
// argument [tierBBindings] and [seedBindings] already make, and the ratchet the
// docRuntime doc comment asks for. It cannot reach the tracer at all.
//
// The store IS here, and deliberately: refuses{}/permits{} evaluate through
// acl.Declarative, which resolves role-relation membership over the seeded
// graph, so a manual can seed the very edge that confers a role and then assert
// what it grants. A policy-only cluster could not do that.
//
// Nil: policy may be nil (a project with no acl.yaml). The descriptive verbs
// then say so and emit nothing; the assertions fail loud, because an assertion
// with no policy to assert against is the vacuous pass this package refuses.
type aclBindings struct {
	// policy is the deployment's ACL policy, or nil when the project has no
	// acl.yaml.
	policy *acl.Policy
	// meta validates that a claim names a declared entity type, and supplies
	// the declared world names worlds_matrix{} lists as columns.
	meta *metamodel.Metamodel
	// store backs the role-relation graph the write assertions evaluate over.
	store store.Store

	// ctx is stored for the same reason the other clusters store one: these are
	// gopher-lua callbacks, which cannot take a context parameter.
	ctx context.Context //nolint:containedctx // request-scoped Lua-binding callbacks

	// emit appends rendered markdown to the current statement island's buffer.
	emit func(string)
	// fail records a typed resolve BuildError on the owning runtime and unwinds
	// the island. A callback rather than a back-pointer to docRuntime, which
	// would recreate the coupling this split removes.
	fail func(ls *lua.LState, format string, args ...any) int
}

// luaFail satisfies [luaFailer] by delegating to the runtime's fail hook, so
// these bindings share rejectUnknownKeys with the rest of the verbs.
func (a *aclBindings) luaFail(ls *lua.LState, format string, args ...any) int {
	return a.fail(ls, format, args...)
}
