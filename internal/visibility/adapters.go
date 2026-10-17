package visibility

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// DeclarativeGate adapts *acl.Declarative to [RowGate]. Every call
// evaluates with the per-operation acl.Request the request edge attached to
// ctx ([acl.Declarative.RequestFrom]). The gate never resolves a principal.
// A ctx with no Request, or with one bound to another principal or
// Declarative, is a gate error, and callers fail closed (deny).
//
// WIRING REQUIREMENT (RR-MXKD2O): every edge resolves the principal once
// and attaches the Request (acl.BindRequest, or the data-entry middleware)
// before calling into the wrappers. The row gate and the redactor then
// evaluate one consistent identity for the whole operation.
type DeclarativeGate struct {
	d *acl.Declarative
	// world is the world a gated hop reads its endpoints in when the
	// surface has no request world: the schema's default world.
	world store.WorldScope
}

// Bind checks that ctx carries a usable acl.Request for this gate and
// returns ctx unchanged. It does not resolve the principal: an unbound ctx
// is an error, and callers deny. The name is kept because it satisfies the
// binder seams in this package ([ScriptReader], [Searcher]).
func (g DeclarativeGate) Bind(ctx context.Context) (context.Context, error) {
	if _, err := g.d.RequestFrom(ctx); err != nil {
		return nil, err
	}
	return ctx, nil
}

// NewDeclarativeGate wraps d (required). world (required) is the schema's
// default world, in which [DeclarativeGate.GateTraversal] reads endpoints.
func NewDeclarativeGate(d *acl.Declarative, world store.WorldScope) (DeclarativeGate, error) {
	if d == nil {
		return DeclarativeGate{}, errors.New("visibility: NewDeclarativeGate: declarative must be non-nil")
	}
	if !world.IsSet() {
		return DeclarativeGate{}, errors.New("visibility: NewDeclarativeGate: world must be set")
	}
	return DeclarativeGate{d: d, world: world}, nil
}

// request returns the acl.Request the edge attached to ctx, or an error
// when there is none; callers fail closed.
func (g DeclarativeGate) request(ctx context.Context) (*acl.Request, error) {
	return g.d.RequestFrom(ctx)
}

// PermitsRead implements [RowGate].
func (g DeclarativeGate) PermitsRead(ctx context.Context, entityType, id string) (bool, error) {
	r, err := g.request(ctx)
	if err != nil {
		return false, err
	}
	return r.PermitsRead(ctx, entityType, id)
}

// ReadableFacesMany implements [RowGate].
func (g DeclarativeGate) ReadableFacesMany(
	ctx context.Context, entityType string, ids []string,
) (acl.FaceVerdicts, error) {
	r, err := g.request(ctx)
	if err != nil {
		return acl.FaceVerdicts{}, err
	}
	return r.ReadableFacesMany(ctx, entityType, ids)
}

// PolicyRedactor adapts *affordances.PolicyResolver to [FieldRedactor]:
// the property names whose FieldVerdicts.Visible entry is an explicit
// false. The resolver returns zero verdicts when no policy is configured,
// so the no-ACL path redacts nothing (NopACL byte-parity); its internal
// failure modes already resolve fail-closed, satisfying the
// [FieldRedactor] contract.
type PolicyRedactor struct {
	r *affordances.PolicyResolver
}

// NewPolicyRedactor wraps r (required).
func NewPolicyRedactor(r *affordances.PolicyResolver) (PolicyRedactor, error) {
	if r == nil {
		return PolicyRedactor{}, errors.New("visibility: NewPolicyRedactor: resolver must be non-nil")
	}
	return PolicyRedactor{r: r}, nil
}

// PrimeTraversals implements [TraversalPrimer].
func (p PolicyRedactor) PrimeTraversals(ctx context.Context, rows []*entity.Entity) context.Context {
	return p.r.PrimeTraversals(ctx, rows)
}

// HiddenProperties implements [FieldRedactor].
func (p PolicyRedactor) HiddenProperties(ctx context.Context, e *entity.Entity) map[string]struct{} {
	v := p.r.FieldVerdicts(ctx, e)
	if len(v.Visible) == 0 {
		return nil
	}
	out := make(map[string]struct{})
	for name, visible := range v.Visible {
		if !visible {
			out[name] = struct{}{}
		}
	}
	return out
}

// SearchFieldScope implements [FieldScoper]: the resolver's `visible:`
// verdict for entityType, in the form the search backend applies.
func (p PolicyRedactor) SearchFieldScope(ctx context.Context, entityType string) (*search.FieldScope, error) {
	return p.r.SearchFieldScope(ctx, entityType)
}

// NopGate is the permit-all [RowGate] for wirings without an ACL policy —
// the byte-parity path (behavior identical to pre-ACL reads).
type NopGate struct{}

// PermitsRead implements [RowGate].
func (NopGate) PermitsRead(context.Context, string, string) (bool, error) { return true, nil }

// ReadableFacesMany implements [RowGate]: every face of every id.
func (NopGate) ReadableFacesMany(context.Context, string, []string) (acl.FaceVerdicts, error) {
	return acl.UniformVerdicts(acl.AllFacesVerdict()), nil
}

// NopRedactor is the hide-nothing [FieldRedactor] for wirings without an
// ACL policy.
type NopRedactor struct{}

// HiddenProperties implements [FieldRedactor].
func (NopRedactor) HiddenProperties(context.Context, *entity.Entity) map[string]struct{} {
	return nil
}

// SearchFieldScope implements [FieldScoper]: nothing is hidden.
func (NopRedactor) SearchFieldScope(context.Context, string) (*search.FieldScope, error) {
	return nil, nil //nolint:nilnil // nil scope is the documented "nothing hidden"
}
