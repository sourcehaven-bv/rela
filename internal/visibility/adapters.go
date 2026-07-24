package visibility

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// DeclarativeGate adapts *acl.Declarative to [RowGate]. The per-principal
// acl.Request is derived FROM CTX on every call — a ctx-attached Request
// (acl.FromContext, the per-request amortization seam) is reused, else a
// fresh one is opened for the ctx principal. An unstamped principal makes
// ForPrincipal fail, which surfaces as a gate error → callers fail
// closed (deny), never open.
type DeclarativeGate struct {
	d *acl.Declarative
}

// NewDeclarativeGate wraps d (required).
func NewDeclarativeGate(d *acl.Declarative) (DeclarativeGate, error) {
	if d == nil {
		return DeclarativeGate{}, errors.New("visibility: NewDeclarativeGate: declarative must be non-nil")
	}
	return DeclarativeGate{d: d}, nil
}

// request resolves the acl.Request for this call: ctx-attached when
// present, else freshly opened for the ctx principal.
func (g DeclarativeGate) request(ctx context.Context) (*acl.Request, error) {
	if r := acl.FromContext(ctx); r != nil {
		return r, nil
	}
	return g.d.ForPrincipal(principal.From(ctx))
}

// PermitsRead implements [RowGate].
func (g DeclarativeGate) PermitsRead(ctx context.Context, entityType, id string) (bool, error) {
	r, err := g.request(ctx)
	if err != nil {
		return false, err
	}
	return r.PermitsRead(ctx, entityType, id)
}

// PermitsReadMany implements [RowGate].
func (g DeclarativeGate) PermitsReadMany(
	ctx context.Context, entityType string, ids []string,
) (map[string]bool, error) {
	r, err := g.request(ctx)
	if err != nil {
		return nil, err
	}
	return r.PermitsReadMany(ctx, entityType, ids)
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

// NopGate is the permit-all [RowGate] for wirings without an ACL policy —
// the byte-parity path (behavior identical to pre-ACL reads).
type NopGate struct{}

// PermitsRead implements [RowGate].
func (NopGate) PermitsRead(context.Context, string, string) (bool, error) { return true, nil }

// PermitsReadMany implements [RowGate].
func (NopGate) PermitsReadMany(_ context.Context, _ string, ids []string) (map[string]bool, error) {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m, nil
}

// NopRedactor is the hide-nothing [FieldRedactor] for wirings without an
// ACL policy.
type NopRedactor struct{}

// HiddenProperties implements [FieldRedactor].
func (NopRedactor) HiddenProperties(context.Context, *entity.Entity) map[string]struct{} {
	return nil
}
