package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
)

// UpdateScope names, per entity type, the properties a [ScopedUpdates] may
// write. It is the `elevated_updates:` block of a data-entry action
// (TKT-JA5SABG).
type UpdateScope map[string][]string

// ScopedUpdates is an elevated update handle limited to an [UpdateScope]. A
// data-entry action that declares `allow_acl_bypass: write` gets one as
// `admin.update_entity`, so its script can set a field its caller may not
// set directly, such as a status that only the action may change.
//
// A write through it skips the row ACL and the field gate, like every write
// under rela.bypass_acl. Three limits keep it narrower than that:
//
//   - The caller must be able to read the target row. A row the caller
//     cannot read is refused like an absent one, so elevation widens which
//     fields the caller may write on rows they see and never reaches others.
//   - The stored type and every named property must be in the scope.
//   - Content and external refs are refused.
//
// Transition rules, validation and automations run as on any update. A
// transition guard still asks whether the caller holds its permission, so a
// field only the action may change is restricted with acl.yaml field rules,
// not with a guard. A refused transition reports no value, because the
// caller may not be able to see the stored one.
//
// Audit: the `acl-bypass` row and the update row name the real principal
// and carry the source label as `triggered_by`. A cascade the write triggers
// runs with normal authority, because the manager dispatches it through a
// non-elevated handle.
type ScopedUpdates struct {
	m      *Manager
	scope  UpdateScope
	source string
}

// NewScopedUpdates returns the handle for scope on m's deps. source labels
// the writes in the audit log, for example "action:indienen".
//
// Nil: rejected — a handle without a manager, a scope or a source would fail
// later at a less obvious place.
func NewScopedUpdates(m *Manager, scope UpdateScope, source string) (*ScopedUpdates, error) {
	if m == nil {
		return nil, errors.New("entitymanager: NewScopedUpdates: nil manager")
	}
	if source == "" {
		return nil, errors.New("entitymanager: NewScopedUpdates: empty source")
	}
	if len(scope) == 0 {
		return nil, errors.New("entitymanager: NewScopedUpdates: empty scope")
	}
	own := make(UpdateScope, len(scope))
	for typ, props := range scope {
		if typ == "" || len(props) == 0 {
			return nil, fmt.Errorf("entitymanager: NewScopedUpdates: type %q has no properties", typ)
		}
		own[typ] = slices.Clone(props)
	}
	return &ScopedUpdates{m: m.gated(), scope: own, source: source}, nil
}

// PatchEntity is [Manager.PatchEntity] on an elevated handle, for a patch
// that stays inside the scope. A target the caller cannot read returns
// [ErrEntityNotFound]; a patch outside the scope returns an error and
// writes nothing.
func (s *ScopedUpdates) PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error) {
	if p.Content != nil {
		return nil, errors.New("elevated update: content is outside the scope")
	}
	if p.WriteExternalRefs {
		return nil, errors.New("elevated update: external refs are outside the scope")
	}
	if len(p.Properties) == 0 && len(p.MetaUnset) == 0 {
		return nil, errors.New("elevated update: no properties to update")
	}
	ctx = audit.WithTriggeredBy(ctx, s.source)
	gate := faceGateOf(s.m)
	check := func(stored *entity.Entity) error {
		// The caller's read gate, before the scope check, so a scope error
		// never confirms that a hidden row exists.
		if err := requireReadableRow(ctx, gate, id, []*entity.Entity{stored}); err != nil {
			if errors.Is(err, ErrEntityNotFound) {
				return newEntityNotFound(id)
			}
			return err
		}
		return s.inScope(stored.Type, p)
	}
	res, err := patchEntity(ctx, s.m.elevated(), id, p, patchPre{check: check})
	return res, valueFreeTransitionError(err)
}

// transitionSentinels are the state-machine failures whose message names the
// stored value.
var transitionSentinels = []error{
	statemachine.ErrIllegalTransition,
	statemachine.ErrGuardDenied,
	statemachine.ErrPreconditionFailed,
	statemachine.ErrIllegalEntry,
}

// valueFreeTransitionError replaces a state-machine failure with a message
// that names no value. A transition error quotes the stored value, and the
// elevated handle skips the field gate that would refuse a write to a
// property the caller cannot see, so the original text could disclose a
// `visible:`-hidden value. The result still matches the sentinel with
// errors.Is, so the failure keeps its status class.
func valueFreeTransitionError(err error) error {
	if err == nil {
		return nil
	}
	// A guard denial reaches here as the 403 that mapTransitionError builds,
	// with the statemachine message as its reason.
	var fe *acl.ForbiddenError
	if errors.As(err, &fe) && fe.Decision.RuleKind == transitionGuardRuleKind {
		slog.Info("elevated update: transition refused", "error", err)
		d := fe.Decision
		d.Reason = "elevated update: transition guard denied"
		return &acl.ForbiddenError{Decision: d}
	}
	for _, sentinel := range transitionSentinels {
		if errors.Is(err, sentinel) {
			slog.Info("elevated update: transition refused", "error", err)
			return fmt.Errorf("elevated update: transition not allowed (%w)", sentinel)
		}
	}
	return err
}

// inScope reports an error when p names a property of typ outside the scope.
func (s *ScopedUpdates) inScope(typ string, p entity.Patch) error {
	allowed, ok := s.scope[typ]
	if !ok {
		return fmt.Errorf("elevated update: entity type %q is outside the scope", typ)
	}
	for name := range p.Properties {
		if !slices.Contains(allowed, name) {
			return fmt.Errorf("elevated update: property %q of %q is outside the scope", name, typ)
		}
	}
	for _, name := range p.MetaUnset {
		if !slices.Contains(allowed, name) {
			return fmt.Errorf("elevated update: property %q of %q is outside the scope", name, typ)
		}
	}
	return nil
}
