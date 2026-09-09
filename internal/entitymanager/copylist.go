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
// presentation. Every offer is SAME-ENTITY (see [CopiesForSource]), so an
// invoke needs only the definition name and the source id.
type CopyOffer struct {
	// Name is the `copies:` key, and the ONLY thing a caller may send back to
	// invoke it. A request never supplies a definition.
	Name string
	// Label is the operator-configured display text, or the definition's name
	// when none is set. Display-only.
	Label string
	// TargetFace is the face this copy writes, as declared (`policy@published`).
	// For a UI that wants to say what the button will do.
	TargetFace string

	// Allowed reports whether this principal may invoke this copy on this
	// source RIGHT NOW.
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
	Allowed bool
	// Reason names why Allowed is false, for a tooltip or a CLI. Empty when
	// Allowed is true. Advisory: it explains a denial the server already made,
	// and never carries content from an entity the caller cannot read.
	Reason string
	// OnSuccess is the definition's operator-declared follow-through (toast
	// text and landing), passed through as declared. Display and navigation
	// only; the kernel never consults it.
	OnSuccess metamodel.CopyOnSuccess
}

// CopiesForSource lists the copy definitions whose `from:` addresses the face
// (entityType, face) of sourceID, each with an invocability verdict.
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
// # Same-entity definitions only
//
// A cross-entity definition (`to: new <type>`) is filtered out BEFORE any
// authorization runs, not probed and reported. Its target does not exist and
// has no id, and authorizeCopy checks OpCreate on EntitySubject{Type, ID,
// Face} — so probing it would authorize the EMPTY id: a confident answer to a
// different question, and the defect a code review caught (list said allowed,
// invoke said forbidden). The source entity in hand does not supply the
// missing id; it is the SOURCE.
//
// The kernel is not narrowed by this. [Manager.CopyState] still performs a
// cross-entity copy for a caller with an explicit target id. Only the
// AFFORDANCE is scoped, because this epic is about faces of ONE entity
// (promote a draft, translate a page). Read it as a scope boundary, not a
// limitation to "complete".
//
// # Ordering
//
// Sorted by name, so a UI renders a stable button order across requests
// rather than one that reshuffles with Go's map iteration.
//
// # Why a package function and not a method
//
// Manager carries a `//plimsoll:max-methods=40` load line, and the project
// rule is to split the type rather than raise the number — so a copy-affordance
// query, which needs only the manager's metamodel and its planning path, does
// not become a forty-first method on the write god-object. The same reasoning
// keeps copyInvocable a function. Consumers that need this as an interface
// method wrap it; see [CopyAffordances].
func CopiesForSource(
	ctx context.Context, m *Manager, entityType, face, sourceID string,
) ([]CopyOffer, error) {
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
		// Compare STORED coordinates, not declared names: the `bare_face` IS
		// the zero face, so `policy@draft` and `policy` address the same face
		// when draft is bare. Comparing the declared strings would offer a
		// promote button on the wrong face.
		if metamodel.StoredFace(m.deps.Meta, from.Type, from.Face) != face {
			continue
		}
		// Filtered before authorization, never probed — see the type doc.
		if !def.IsSameEntity() {
			continue
		}

		offer := CopyOffer{
			Name:       name,
			Label:      copyOfferLabel(name, def),
			TargetFace: def.To,
			OnSuccess:  def.OnSuccess,
		}
		offer.Allowed, offer.Reason = copyInvocable(ctx, m, name, def, sourceID)
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
// planCopy reads the source through the same gate the invoke uses and calls
// authorizeCopy itself (copy.go, right after readCopySource), which applies
// the read gate and the definition's guard. Anything those refuse is refused
// here identically, because it is the same code — and because there is
// exactly ONE way to plan a copy and it is inseparable from authorizing it,
// this function does not call authorizeCopy a second time. An earlier revision
// did; a mutation check showed deleting the second call changed nothing.
//
// # Every non-nil error is "not allowed", and that is deliberate
//
// A missing source, a failed guard, an unparseable definition — from a UI's
// point of view these are one answer: do not render the button. Distinguishing
// them here would mean deciding which failures are safe to describe, and the
// read gate's whole design is that a denied source is indistinguishable from
// an absent one. So the Reason is the error's own text for a genuine
// authorization refusal, and a generic phrase otherwise.
func copyInvocable(
	ctx context.Context, m *Manager, name string, def metamodel.CopyDef, sourceID string,
) (allowed bool, reason string) {
	// Mark the context so the authorization path computes its verdict WITHOUT
	// auditing it: this is a question, not an attempted write. See
	// [withAffordanceProbe].
	ctx = withAffordanceProbe(ctx)

	ce := &copyEngine{m: m}
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
	if forbidden, ok := errors.AsType[*acl.ForbiddenError](err); ok {
		return forbidden.Decision.Reason
	}
	return "not available for this entity"
}

// CopyAffordances adapts a [Manager] to the method-shaped copy surface a
// consumer-side interface needs: `internal/dataentry` declares a narrow
// interface at its call site, per the project's interfaces-at-the-consumer
// rule, while [CopiesForSource] is a package function (see its doc for why).
//
// It carries no state and makes no decisions — in particular it does NOT
// authorize. Every method forwards to the manager, which authorizes
// internally. A wrapper that grew a check of its own would be the second
// authorization site this design exists to avoid.
type CopyAffordances struct{ M *Manager }

// CopiesForSource forwards to [CopiesForSource].
func (c CopyAffordances) CopiesForSource(
	ctx context.Context, entityType, face, sourceID string,
) ([]CopyOffer, error) {
	return CopiesForSource(ctx, c.M, entityType, face, sourceID)
}

// CopyState forwards to [Manager.CopyState], which authorizes internally.
func (c CopyAffordances) CopyState(ctx context.Context, req CopyRequest) (*CopyResult, error) {
	return c.M.CopyState(ctx, req)
}
