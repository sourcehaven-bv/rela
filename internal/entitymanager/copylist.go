package entitymanager

import (
	"context"
	"errors"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// CopyOffer is one copy definition offered for a given source face, with
// whether this principal may invoke it right now (TKT-WRLDAPI item 5).
//
// It is the vocabulary a UI needs to render RULING 9's affordances — one
// button per definition whose source is the face being viewed, hidden when
// the principal lacks the guard — without deciding anything about
// presentation.
type CopyOffer struct {
	// Name is the `copies:` key, and the ONLY thing a caller may send back to
	// invoke it. A request never supplies a definition.
	Name string
	// Label is the operator-configured display text, or the definition's name
	// when none is set. Display-only.
	Label string
	// TargetFace is the face this copy writes, as declared (`policy@published`
	// or `new page`). For a UI that wants to say what the button will do.
	TargetFace string
	// SameEntity distinguishes a promote/revise (writes another face of the
	// SAME entity) from a copy that creates a different entity. A cross-entity
	// copy needs a target id, so a caller must know which it is looking at.
	SameEntity bool

	// Indeterminate marks an offer whose invocability CANNOT be answered
	// without information the caller has not supplied yet.
	//
	// It is true for exactly one case: a CROSS-ENTITY copy, whose target id
	// the client chooses at invoke time. The authorization path checks
	// `OpCreate` on the target, so with no target id it would authorize the
	// EMPTY id — and any policy whose verdict depends on the target's identity
	// would be evaluated against the wrong subject. A code-review reproduction
	// showed exactly that: list said allowed, invoke said forbidden.
	//
	// Reporting `Allowed: true` for that case would be the RULING 11 defect —
	// an affordance that says one thing while the write does another. So the
	// honest answer is "I cannot know", and this field says so rather than
	// guessing. A client should render such an offer as available-but-
	// unverified (the server still authorizes), never as confirmed.
	//
	// Same-entity offers are always determinate: their target IS the source.
	Indeterminate bool

	// Allowed reports whether this principal may invoke this copy on this
	// source RIGHT NOW.
	//
	// Meaningful only when Indeterminate is false. See that field.
	//
	// # It is a HINT, never a boundary
	//
	// Same contract as the `_actions` affordance map: the invoke endpoint
	// re-authorizes through the kernel, and a caller that ignores this and
	// POSTs anyway gets the same 403 it would have got regardless. A UI uses
	// it to decide what to render; nothing downstream may rely on it.
	//
	// It is computed by calling the SAME authorization path the invoke uses
	// ([Manager.authorizeCopy] behind [Manager.planCopy]), not by a parallel
	// reimplementation of the guard check. Two authorization computations that
	// can disagree is a worse defect than having no hint at all — that is the
	// RULING 11 failure, where an affordance map said "writable" and the write
	// path refused.
	//
	// That guarantee holds for SAME-ENTITY offers. For a cross-entity offer
	// the inputs differ (no target id yet), which is what Indeterminate
	// records — the mechanism is shared, but the question is not the same one.
	Allowed bool
	// Reason names why Allowed is false, for a tooltip or a CLI. Empty when
	// Allowed is true. Advisory: it explains a denial the server already made,
	// and never carries content from an entity the caller cannot read.
	Reason string
}

// CopiesForSource lists the copy definitions whose `from:` addresses the face
// (entityType, pointer) of sourceID, each with an invocability verdict.
//
// # Why it lists definitions the caller may NOT invoke
//
// Copy definition NAMES are operator-authored config — they live in
// schema.yaml, routinely in a public repo — so which definitions exist is not
// a secret, and filtering the list to conceal them would be concealment of
// something already disclosed. What IS per-principal is whether this caller
// may invoke one HERE, and that rides on each entry as [CopyOffer.Allowed].
//
// Contrast the entity read gate, which does hide existence: whether an ENTITY
// exists is a genuine secret. A definition is config; a row is data.
//
// # Ordering
//
// Sorted by name, so a UI renders a stable button order across requests
// rather than one that reshuffles with Go's map iteration.
//
// A package FUNCTION, not a method. Manager carries a
// `//plimsoll:max-methods=40` load line, and the project rule is to split the
// type rather than raise the number — so a copy-affordance query, which needs
// only the manager's metamodel and its planning path, does not become a
// forty-first method on the write god-object.
//
// Consumers that need this as an interface method wrap it; see
// [CopyAffordances].
func CopiesForSource(
	ctx context.Context, m *Manager, entityType, pointer, sourceID string,
) ([]CopyOffer, error) {
	if m.deps.Meta == nil {
		return nil, nil
	}
	names := make([]string, 0, len(m.deps.Meta.Copies))
	for name := range m.deps.Meta.Copies {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []CopyOffer
	for _, name := range names {
		def := m.deps.Meta.Copies[name]
		from, err := metamodel.ParseCopyTarget(def.From)
		if err != nil {
			// A definition that does not parse cannot match anything. Load-time
			// validation already refuses these, so this is unreachable in a
			// loaded metamodel; skipping rather than erroring keeps one bad
			// definition from blanking the whole list.
			continue
		}
		if from.Type != entityType {
			continue
		}
		// Compare STORED coordinates, not declared names: a pointer marked
		// `default: true` IS the zero pointer, so `policy@draft` and `policy`
		// address the same face when draft is the default. Comparing the
		// declared strings would offer a promote button on the wrong face.
		if metamodel.StoredPointer(m.deps.Meta, from.Type, from.Pointer) != pointer {
			continue
		}

		offer := CopyOffer{
			Name:       name,
			Label:      copyOfferLabel(name, def),
			TargetFace: def.To,
			SameEntity: def.IsSameEntity(),
		}
		if offer.SameEntity {
			offer.Allowed, offer.Reason = copyInvocable(ctx, m, name, def, sourceID)
		} else {
			// A cross-entity copy's target is chosen at invoke time, so its
			// authorization cannot be evaluated now — see CopyOffer.Indeterminate.
			// Deliberately NOT probed at all: running the check against an empty
			// target id would produce a confident answer to a different question.
			offer.Indeterminate = true
		}
		out = append(out, offer)
	}
	return out, nil
}

// copyOfferLabel resolves the display text: the operator's `label:`, or the
// definition's name when none is set.
//
// The name is a legible fallback because copy names are operator-authored and
// read as actions already (`promote-policy`, `translate-to-nl`) — the same
// reasoning [metamodel.Transition.Label] uses when it falls back to the target
// value.
func copyOfferLabel(name string, def metamodel.CopyDef) string {
	if def.Label != "" {
		return def.Label
	}
	return name
}

// copyInvocable answers whether this principal may invoke def on sourceID,
// by running the REAL authorization path rather than re-deriving it.
//
// planCopy reads the source through the same gate the invoke uses and
// authorizeCopy applies the read gate, the definition's guard and (for a
// cross-entity copy) the create check. Anything those refuse is refused here
// identically, because it is the same code.
//
// # Every non-nil error is "not allowed", and that is deliberate
//
// A missing source, a failed guard, an unparseable definition — from a UI's
// point of view these are one answer: do not render the button. Distinguishing
// them here would mean deciding which failures are safe to describe, and the
// read gate's whole design is that a denied source is indistinguishable from
// an absent one. So the Reason is the error's own text for a genuine
// authorization refusal, and a generic phrase otherwise.
//
// A package FUNCTION rather than a method, deliberately. Manager carries a
// `//plimsoll:max-methods=40` load line and CopiesForSource + this would have
// taken it to 42; the load line is a ratchet to narrow, not a budget to spend.
// CopiesForSource STAYS a method because the consumer-side interface in
// internal/dataentry is satisfied structurally by the concrete manager, and a
// package function could not participate in that. This one needs nothing from
// the receiver that a parameter cannot carry, so it moves.
func copyInvocable(
	ctx context.Context, m *Manager, name string, def metamodel.CopyDef, sourceID string,
) (allowed bool, reason string) {
	// Mark the context so the authorization path computes its verdict WITHOUT
	// auditing it: this is a question, not an attempted write. See
	// [withAffordanceProbe].
	ctx = withAffordanceProbe(ctx)

	ce := &copyEngine{m: m}

	// planCopy AUTHORIZES INTERNALLY — it calls authorizeCopy itself (copy.go,
	// right after readCopySource), which is why this does not call it again.
	//
	// An earlier revision did, and a mutation check exposed the redundancy:
	// deleting the second call changed nothing, because the first had already
	// run. That is the stronger arrangement, not the weaker one — there is
	// exactly ONE way to plan a copy and it is inseparable from authorizing
	// it, so a caller cannot obtain a plan without a verdict. A second call
	// here would have implied the authorization was this function's to
	// arrange, which would invite someone to "optimize" it away.
	if _, err := ce.planCopy(ctx, CopyRequest{Definition: name, SourceID: sourceID}, def); err != nil {
		return false, copyDenialReason(err)
	}
	return true, ""
}

// copyDenialReason renders a denial for a UI tooltip.
//
// An acl.ForbiddenError already carries operator-facing prose naming the
// permission, which is useful and discloses only config. Anything else — a
// missing source, a store fault — collapses to one phrase: the alternative is
// leaking whether an entity the caller cannot read exists.
func copyDenialReason(err error) string {
	var forbidden *acl.ForbiddenError
	if errors.As(err, &forbidden) {
		return forbidden.Decision.Reason
	}
	return "not available for this entity"
}

// CopyAffordances adapts a [Manager] to the method-shaped copy surface a
// consumer-side interface needs.
//
// It exists because [CopiesForSource] is a package function (Manager is at its
// plimsoll load line) while `internal/dataentry` declares a narrow
// `copyService` interface at its call site, per the project's
// interfaces-at-the-consumer rule. This is the one line that reconciles the
// two: a value type wrapping the manager, whose methods delegate.
//
// It carries no state and makes no decisions — in particular it does NOT
// authorize. Every method here forwards to the manager, which authorizes
// internally. A wrapper that grew a check of its own would be the second
// authorization site this design exists to avoid.
type CopyAffordances struct{ M *Manager }

// CopiesForSource forwards to [CopiesForSource].
func (c CopyAffordances) CopiesForSource(
	ctx context.Context, entityType, pointer, sourceID string,
) ([]CopyOffer, error) {
	return CopiesForSource(ctx, c.M, entityType, pointer, sourceID)
}

// CopyState forwards to [Manager.CopyState], which authorizes internally.
func (c CopyAffordances) CopyState(ctx context.Context, req CopyRequest) (*CopyResult, error) {
	return c.M.CopyState(ctx, req)
}
