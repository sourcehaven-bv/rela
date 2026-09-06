package predicatefns

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// Names of the current-user surface. Exported so generated predicate
// source (FromFilter, and any future sugar) references them without
// stringly-typed drift, exactly as the stdlib names above do.
const (
	// VarCurrentUser is the record variable naming the request's identity.
	VarCurrentUser = "current_user"

	// VarEntity is the record variable naming the entity under test. It
	// is declared by the Evaluator rather than here, but the name is
	// exported alongside VarCurrentUser so a pushdown caller naming both
	// records reads them from one place.
	VarEntity = "entity"

	// FieldCurrentUserID is the identity field of [VarCurrentUser] — the
	// only field a pushdown may resolve to a value.
	FieldCurrentUserID = "id"

	// FuncIsCurrentUser reports whether a string equals the current
	// user's query identity: is_current_user(entity.assignee).
	FuncIsCurrentUser = "is_current_user"

	// FuncHasCurrentUser reports whether the current user's query
	// identity appears in a list of strings:
	// has_current_user(entity.watchers).
	FuncHasCurrentUser = "has_current_user"
)

// CurrentUserType is the declared shape of [VarCurrentUser].
//
// It is a RECORD, not a bare string, and that is a compatibility
// constraint rather than a preference: operator-authored acl.yaml
// already passes the variable whole to affordance host functions
// (`has_role(current_user, entity, "editor")`), and internal/affordances
// declares it as predicate.RecordType. Redeclaring it as a string here
// would make the same identifier mean two different types in two
// dialects an operator experiences as one language.
//
// The fields mirror internal/affordances/env.go so a `when:` clause and
// a query condition read identically:
//
//   - id   — the query identity (see [QueryIdentity]).
//   - tool — the entry-point tool ("data-entry", "mcp", "cli", ...).
//
// As in affordances, `tool` is NOT a user classification. It exists for
// diagnostics and transport-shaped conditions; authorization gates by
// role, never by inspecting it.
var CurrentUserType = predicate.RecordType{
	"id":   predicate.StringType,
	"tool": predicate.StringType,
}

// ErrNoCurrentUser is returned by [BindCurrentUser] when the context
// carries no usable query identity.
//
// It is an ERROR rather than a nil/empty binding by deliberate design.
// An empty identity would make `entity.assignee == current_user.id`
// match every entity whose assignee is unset — turning "who am I?"
// being unanswerable into a filter that silently WIDENS. Every other
// direction in this codebase fails closed (acl.ResolvePrincipal returns
// no id on ambiguity; an unparseable where: clause is a load error), and
// a personal inbox that shows a stranger's rows is the worst version of
// getting this wrong.
var ErrNoCurrentUser = errors.New("predicatefns: no current user on this context")

// QueryIdentity is the value `current_user.id` binds to.
//
// # Why a distinct type
//
// The string a principal carries is NOT stable across transports.
// internal/dataentry rewrites principal.User to the resolved user-entity
// id on /api/ (resolvePrincipalEntity), while CLI, MCP, scheduler and
// desktop keep the raw identifier — acl.Declarative.ResolvePrincipal
// documents that split and its fail-closed rationale. A query condition
// compares against entity DATA (`entity.assignee`, which holds an entity
// id), so binding whichever string happened to be on the context would
// make the same view correct in the browser and silently empty from the
// CLI.
//
// So the identity is resolved ONCE, by the wiring site that knows the
// ACL policy, and carried on the context as this type. Consumers bind it
// verbatim; they never re-derive it from principal.User.
type QueryIdentity struct {
	// EntityID is the user entity this principal resolves to. Empty when
	// the deployment configures no user_entity_type, in which case there
	// is no entity id to compare against and Raw is used instead.
	EntityID string

	// Raw is the underlying principal identifier (an email, a UPN, a
	// service account name).
	Raw string

	// Tool is the entry-point tool, bound to current_user.tool.
	Tool string
}

// ID returns the value bound to current_user.id: the resolved user
// entity id when the deployment has one, else the raw principal.
//
// Preferring the entity id is what makes `entity.assignee ==
// current_user.id` mean what an operator reads it to mean, since an
// assignee property holds an entity id. Falling back to the raw
// identifier keeps the feature usable in deployments with no user
// entity type at all (the data-entry prototype is one), where the raw
// string IS the only identity the graph could reference.
func (q QueryIdentity) ID() string {
	if q.EntityID != "" {
		return q.EntityID
	}
	return q.Raw
}

// Valid reports whether this identity can be compared against graph
// data. A zero QueryIdentity is invalid, which is what makes the
// unstamped-context case fail closed.
func (q QueryIdentity) Valid() bool { return q.ID() != "" }

type queryIdentityKey struct{}

// WithQueryIdentity stamps the resolved identity on ctx.
//
// Call it at a REQUEST boundary that has already resolved the principal
// against the ACL policy. It is deliberately not derived lazily inside
// the evaluator: resolution is a store lookup, and doing it per matched
// entity would turn one list render into one query per row.
func WithQueryIdentity(ctx context.Context, q QueryIdentity) context.Context {
	return context.WithValue(ctx, queryIdentityKey{}, q)
}

// QueryIdentityFrom returns the identity stamped on ctx, if any.
//
// Absence is reported rather than defaulted. principal.From supplies an
// "unknown/unknown" default for attribution, which is right for an audit
// record and wrong here: "unknown" is not a user id, and binding it
// would let a condition match an entity whose assignee is literally
// "unknown".
func QueryIdentityFrom(ctx context.Context) (QueryIdentity, bool) {
	q, ok := ctx.Value(queryIdentityKey{}).(QueryIdentity)
	if !ok || !q.Valid() {
		return QueryIdentity{}, false
	}
	return q, true
}

// DeclareCurrentUser registers the current-user variable and its sugar
// functions on env.
//
// Kept SEPARATE from [Declare] rather than folded into it, because the
// stdlib is safe everywhere and this is not. Validation compiles with
// context.Background() (internal/validation), the scheduler and mail
// templates compile at load with no request in sight, and index
// derivation compiles a query shape with no user at all. Those profiles
// must NOT declare current_user: referencing it there is an operator
// mistake, and an undeclared variable is a COMPILE error naming the line
// — which is the failure an operator can act on, and the direction
// CLAUDE.md mandates for a condition that cannot be honored.
//
// Call it after DeclareVar("entity", ...) and before Compile, alongside
// [Declare].
func DeclareCurrentUser(env *predicate.Env) error {
	if err := env.DeclareVar(VarCurrentUser, CurrentUserType); err != nil {
		return fmt.Errorf("predicatefns: declare %s: %w", VarCurrentUser, err)
	}
	str := predicate.StringType
	decls := []struct {
		name string
		sig  predicate.FuncSig
	}{
		// Both are SQL-portable: each compares stored data against a
		// scalar that is CONSTANT for the request, which is exactly the
		// shape a pushdown binds as a query parameter. Classifying them
		// portable is what lets a future predicate->SQL compiler push
		// `is_current_user(entity.assignee)` down as `assignee = $1`; it does not
		// by itself perform any pushdown.
		{FuncIsCurrentUser, predicate.FuncSig{
			Params: []predicate.Type{str}, Return: predicate.BoolType, SQLPortable: true,
		}},
		{FuncHasCurrentUser, predicate.FuncSig{
			Params: []predicate.Type{predicate.ListType{Elem: predicate.StringType}},
			Return: predicate.BoolType, SQLPortable: true,
		}},
	}
	for _, d := range decls {
		if err := env.DeclareFunc(d.name, d.sig); err != nil {
			return fmt.Errorf("predicatefns: declare %s: %w", d.name, err)
		}
	}
	return nil
}

// BindCurrentUser binds the variable and sugar functions declared by
// [DeclareCurrentUser], reading the identity from ctx.
//
// Returns [ErrNoCurrentUser] when ctx carries none. A caller that
// compiled a program referencing current_user and then cannot bind it
// has a wiring bug, and surfacing it beats evaluating a condition whose
// identity is a guess.
func BindCurrentUser(ctx context.Context, b *predicate.Bindings) error {
	q, ok := QueryIdentityFrom(ctx)
	if !ok {
		return ErrNoCurrentUser
	}
	identity := q.ID()
	if err := b.SetVar(VarCurrentUser, predicate.NewRecord(map[string]predicate.Value{
		"id":   predicate.NewString(identity),
		"tool": predicate.NewString(q.Tool),
	})); err != nil {
		return err
	}
	binds := []struct {
		name string
		fn   predicate.FuncFunc
	}{
		{FuncIsCurrentUser, isCurrentUser(identity)},
		{FuncHasCurrentUser, hasCurrentUser(identity)},
	}
	for _, bd := range binds {
		if err := b.SetFunc(bd.name, bd.fn); err != nil {
			return fmt.Errorf("predicatefns: bind %s: %w", bd.name, err)
		}
	}
	return nil
}

// isCurrentUser implements is_current_user(s).
//
// A Nil argument (an unset property binds Nil, per coerceScalar) is a
// non-match rather than an error: is_current_user(entity.assignee) on an
// unassigned entity is a legitimate question with the answer "no". Only
// a genuinely off-type argument fails, which the type checker should
// already have rejected.
func isCurrentUser(identity string) predicate.FuncFunc {
	return func(_ context.Context, args []predicate.Value) (predicate.Value, error) {
		if len(args) != 1 {
			return nil, errArg
		}
		switch v := args[0].(type) {
		case predicate.Nil:
			return predicate.NewBool(false), nil
		case predicate.String:
			// identity is non-empty (QueryIdentity.Valid gates the
			// binding), so this cannot degenerate into matching every
			// entity whose property is unset.
			return predicate.NewBool(v.String() == identity), nil
		default:
			return nil, errArg
		}
	}
}

// hasCurrentUser implements has_current_user(list).
//
// Named for the LIST, not the user: it reads as a question about the
// property being tested ("do the watchers include the current user?"),
// which is the direction an operator is thinking in when they write it.
func hasCurrentUser(identity string) predicate.FuncFunc {
	return func(_ context.Context, args []predicate.Value) (predicate.Value, error) {
		if len(args) != 1 {
			return nil, errArg
		}
		switch v := args[0].(type) {
		case predicate.Nil:
			return predicate.NewBool(false), nil
		case predicate.List:
			for _, e := range v.Elems() {
				if s, ok := e.(predicate.String); ok && s.String() == identity {
					return predicate.NewBool(true), nil
				}
			}
			return predicate.NewBool(false), nil
		default:
			return nil, errArg
		}
	}
}

// PrincipalResolver maps a raw principal identifier to the user entity
// that represents it, or "" when the deployment configures no user
// entity type or the identifier matches none.
//
// Consumer-side interface (see docs/architecture/consumer-side-interfaces.md):
// the one implementation is acl.Declarative.ResolvePrincipal, but
// predicatefns must not depend on internal/acl — and the one method it
// needs is far narrower than that type's surface. The wiring site
// supplies it.
//
// Nil: accepted by [ResolveQueryIdentity], which then treats the
// deployment as having no user entity type.
type PrincipalResolver interface {
	ResolvePrincipal(ctx context.Context, rawUser string) (string, error)
}

// ResolveQueryIdentity builds the [QueryIdentity] for the principal
// carried on ctx, resolving it through r when one is supplied.
//
// This is the single place a transport turns "who is calling" into "what
// string do I compare against graph data", so every surface agrees. Call
// it once per request at the boundary and stamp the result with
// [WithQueryIdentity]; do not call it per row.
//
// A resolver error is returned rather than swallowed. The caller decides
// whether to fail the request or proceed without an identity — but an
// identity that silently degrades to the raw principal after a backend
// error would compare against a DIFFERENT namespace than the one the
// operator wrote the condition for, and match the wrong rows rather than
// none.
//
// An unstamped context yields an invalid identity (not an error): "no
// identity" is a legitimate state for the CLI and the scheduler, and it
// is [BindCurrentUser] that refuses to evaluate against it.
func ResolveQueryIdentity(
	ctx context.Context, r PrincipalResolver, rawUser, tool string,
) (QueryIdentity, error) {
	q := QueryIdentity{Raw: strings.TrimSpace(rawUser), Tool: tool}
	if q.Raw == "" || r == nil {
		return q, nil
	}
	id, err := r.ResolvePrincipal(ctx, q.Raw)
	if err != nil {
		return QueryIdentity{}, fmt.Errorf("predicatefns: resolve current user: %w", err)
	}
	// id == raw means "resolved to itself", which carries no more
	// information than Raw already does; leaving EntityID empty keeps
	// ID()'s fallback the single explanation of the value.
	if id != "" && id != q.Raw {
		q.EntityID = id
	}
	return q, nil
}
