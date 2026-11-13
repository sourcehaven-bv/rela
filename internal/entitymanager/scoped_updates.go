package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// UpdateScope names the properties a [ScopedUpdates] may write. It is the
// `elevated_updates:` block of a data-entry action (TKT-JA5SABG). A key is
// the canonical type name for a type without faces, and `type@face` for one
// face of a faced type: a faced type has no row at the implicit face, so a
// bare key never matches it.
type UpdateScope map[string][]string

// ScopedUpdates is an elevated update handle limited to an [UpdateScope]. A
// data-entry action that declares `allow_acl_bypass: write` gets one as
// `admin.update_entity`, so its script can set a field its caller may not
// set directly, such as a status that only the action may change.
//
// The elevation is one thing: the write skips the row-level write check
// (the `update:` grants in acl.yaml). acl.yaml field write rules do not
// apply to data-entry action scripts at all, elevated or not, because an
// action writes through the manager handle that skips the field gate. These
// limits keep the elevated write narrow:
//
//   - The caller must be able to read the target row. A row the caller
//     cannot read is refused like an absent one, so elevation widens which
//     rows the caller may update among the rows they see and never reaches
//     others.
//   - The stored type (and face) and every named property must be in the
//     scope.
//   - Content and external refs are refused.
//   - Every check of the ACL's AuthorizeWrite that does not come from a role
//     grant still applies, through [acl.AuthorizePreRoleWrite]: read-only
//     mode, a usable identity, `unmatched_principal: reject`, and the client
//     ceiling (client_baselines and scope_grants) on update of the stored
//     type. Elevation lifts the role grants, never these. A refusal is the
//     ACL's 403 with a denied-write audit row.
//
// Transition rules and validation run as on any update. A transition guard
// still asks whether the caller holds its permission. A refused transition
// reports no value, because the caller may not be able to see the stored
// one.
//
// On-update automations of the target's own type run inside the elevated
// write: a `set` they make on the same entity is stored with it and is not
// checked against the scope. A cascade the write triggers, such as
// `create_entity` or `run_script`, runs with the caller's normal access,
// because the manager dispatches it through a non-elevated handle.
//
// Audit: the `acl-bypass` row and the update row name the real principal
// and carry the source label as `triggered_by`.
type ScopedUpdates struct {
	m      *Manager
	scope  UpdateScope
	source string

	// mu guards pin. A Lua state is single-goroutine, but the handle does
	// not rely on that.
	mu  sync.Mutex
	pin *pinnedRow
}

// pinnedRow is the raw version of the row an action was started on.
type pinnedRow struct {
	ref     entity.Ref
	version store.EntityVersion
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
	for key, props := range scope {
		if key == "" || len(props) == 0 {
			return nil, fmt.Errorf("entitymanager: NewScopedUpdates: type %q has no properties", key)
		}
		own[key] = slices.Clone(props)
	}
	return &ScopedUpdates{m: m.gated(), scope: own, source: source}, nil
}

// Pin makes an elevated write to row's entity conditional on row being
// unchanged. row is the raw stored row the action was started on, which the
// action's `when:` and the script's checks judged. A concurrent change to it
// then fails the write instead of applying on top. After a successful
// elevated write the pin moves to the written row, so a script may update
// the same entity twice. A normal write by the script to that entity in
// between also fails the next elevated write, so a script makes its elevated
// write first.
//
// Nil: accepted, and the handle then pins nothing.
func (s *ScopedUpdates) Pin(row *entity.Entity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row == nil {
		s.pin = nil
		return
	}
	s.pin = &pinnedRow{ref: row.Ref(), version: store.VersionOf(row)}
}

// PatchEntity is [Manager.PatchEntity] on an elevated handle, for a patch
// that stays inside the scope. id is the resolved address of one row, `ID`
// or `ID@face`, as the caller's write-target resolution returns it. A target
// the caller cannot read returns [ErrEntityNotFound]; a patch outside the
// scope returns an error and writes nothing.
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
		if err := s.inScope(stored, p); err != nil {
			return err
		}
		if err := s.preRoleRefusal(ctx, stored); err != nil {
			return err
		}
		return s.pinRefusal(stored)
	}
	res, err := patchEntity(ctx, s.m.elevated(), id, p, patchPre{check: check})
	if err == nil && res != nil {
		s.advancePin(res.Entity)
	}
	return res, valueFreeTransitionError(err)
}

// preRoleRefusal refuses the write when it fails a check of the ACL's
// AuthorizeWrite that does not come from a role grant: read-only mode, a
// missing or mismatched identity, `unmatched_principal: reject`, or the
// client ceiling. Elevation skips AuthorizeWrite, so the scoped write asks
// [acl.AuthorizePreRoleWrite], which applies the same checks through the
// same functions. It never asks a role grant: the grants are what elevation
// lifts. A refusal is the ACL's 403 with a denied-write audit row, as on a
// normal update.
func (s *ScopedUpdates) preRoleRefusal(ctx context.Context, stored *entity.Entity) error {
	req := entityUpdateRequest(stored.Type, stored.Ref())
	decision := acl.AuthorizePreRoleWrite(ctx, s.m.deps.ACL, req)
	if decision.Allow {
		return nil
	}
	if !isAffordanceProbe(ctx) {
		s.m.deps.Audit.Record(deniedWriteRecord(ctx, decision, req))
	}
	return &acl.ForbiddenError{Decision: decision}
}

// errPinnedRowChanged refuses an elevated write to an entity that changed
// since the action read it. It matches [store.ErrConflict] but is not a
// [*store.VersionConflictError], so the patch is not retried, and it names
// no version, so it reveals nothing about hidden fields.
var errPinnedRowChanged = fmt.Errorf("elevated update: the entity changed since the action started: %w",
	store.ErrConflict)

// pinRefusal refuses the write when stored is the pinned row and its version
// moved. The store write itself is pinned to the version of stored, so a
// change that lands after this check fails the store's compare-and-swap and
// the retry runs this check again.
func (s *ScopedUpdates) pinRefusal(stored *entity.Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pin == nil || stored.Ref() != s.pin.ref {
		return nil
	}
	if store.VersionOf(stored) != s.pin.version {
		return errPinnedRowChanged
	}
	return nil
}

// advancePin moves the pin to written when written is the pinned row.
func (s *ScopedUpdates) advancePin(written *entity.Entity) {
	if written == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pin != nil && written.Ref() == s.pin.ref {
		s.pin.version = store.VersionOf(written)
	}
}

// transitionSentinels are the state-machine failures whose message names the
// stored value.
var transitionSentinels = []error{
	flowcfg.ErrIllegalTransition,
	flowcfg.ErrGuardDenied,
	flowcfg.ErrPreconditionFailed,
	flowcfg.ErrIllegalEntry,
}

// valueFreeTransitionError replaces a state-machine failure with a message
// that names no value. A transition error quotes the stored value, and no
// field gate refuses an action's write to a property the caller cannot see,
// so the original text could disclose a `visible:`-hidden value. The result still matches the sentinel with
// errors.Is, so the failure keeps its status class. The log names only the
// rule kind or the sentinel, for the same reason.
func valueFreeTransitionError(err error) error {
	if err == nil {
		return nil
	}
	// A guard denial reaches here as the 403 that mapTransitionError builds,
	// with the transition enforcer message as its reason.
	var fe *acl.ForbiddenError
	if errors.As(err, &fe) && fe.Decision.RuleKind == transitionGuardRuleKind {
		slog.Info("elevated update: transition refused", "rule_kind", fe.Decision.RuleKind)
		d := fe.Decision
		d.Reason = "elevated update: transition guard denied"
		return &acl.ForbiddenError{Decision: d}
	}
	for _, sentinel := range transitionSentinels {
		if errors.Is(err, sentinel) {
			slog.Info("elevated update: transition refused", "reason", sentinel.Error())
			return fmt.Errorf("elevated update: transition not allowed (%w)", sentinel)
		}
	}
	return err
}

// scopeKey is the [UpdateScope] key that covers e: its type, and for a row
// at a named face, `type@face`.
func scopeKey(e *entity.Entity) string {
	face := e.Ref().Face
	if face.IsImplicit() {
		return e.Type
	}
	return e.Type + entity.StateRefSeparator + face.String()
}

// inScope reports an error when p names a property of stored outside the
// scope.
func (s *ScopedUpdates) inScope(stored *entity.Entity, p entity.Patch) error {
	key := scopeKey(stored)
	allowed, ok := s.scope[key]
	if !ok {
		return fmt.Errorf("elevated update: entity type %q is outside the scope", key)
	}
	for name := range p.Properties {
		if !slices.Contains(allowed, name) {
			return fmt.Errorf("elevated update: property %q of %q is outside the scope", name, key)
		}
	}
	for _, name := range p.MetaUnset {
		if !slices.Contains(allowed, name) {
			return fmt.Errorf("elevated update: property %q of %q is outside the scope", name, key)
		}
	}
	return nil
}
