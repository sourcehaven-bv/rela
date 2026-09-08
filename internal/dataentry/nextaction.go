package dataentry

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// nextActionCandidates adapts the app's ACL-gated read paths into the
// [nextaction.CandidateFunc] the engine takes.
//
// # Why this lives here and not in internal/nextaction
//
// Same reason the list-table renderer lives in dataentry rather than
// internal/transform: the engine must never see an entity the request
// principal cannot read, and the read gate is resolved per-request from the
// context here. Keeping the adapter at this layer means the engine depends on
// no store, searcher or ACL type, and cannot accidentally acquire an ungated
// handle.
//
// # Which seam each source kind uses
//
//   - Query  -> App.executeQuery, the same helper /_search and scope
//     navigation use. It resolves SearchScope from the read gate first, and
//     for a query with no free text (which every structural next-action query
//     is) routes to visibleListByTypes — a type-scoped store list, NOT the
//     search index. So the "SearchVisible does not push filters down" caveat
//     does not apply on this path.
//   - Count  -> GraphCount over the type, for the entity-less first-run case.
//     Uses the ungated count deliberately: see countCandidates.
//   - Context -> not reachable from the dashboard; a context-aware source is
//     resolved against the entity being viewed, which is a later surface.
func (a *App) nextActionCandidates() nextaction.CandidateFunc {
	return func(ctx context.Context, src dataentryconfig.NextActionSource) ([]nextaction.Candidate, error) {
		// The SOURCE world is resolved per source, before any read, and
		// REPLACES whatever world the request carried. See
		// nextActionSourceWorld for why the caller's `?world=` must not reach
		// a candidate query.
		ctx, ok, err := nextActionSourceWorld(ctx, a.worlds, src)
		if err != nil {
			return nil, err
		}
		if !ok {
			// This principal holds no read grant for the source's world.
			// Contribute nothing — deliberately a skip, not an error.
			return nil, nil
		}
		switch {
		case src.Count != "":
			return a.countCandidates(ctx, src.Count, src.CountUngated)
		case src.Query != "":
			return a.queryCandidates(ctx, src.Query)
		default:
			// A context-aware source has no dashboard candidates. Not an
			// error: the same config serves both surfaces, and each ignores
			// the other's sources.
			return nil, nil
		}
	}
}

// nextActionSourceWorld rebinds ctx to the world a source's QUERY runs in,
// reporting ok=false when this principal may not read that world.
//
// # The source world REPLACES the request's world, never inherits it
//
// A next-action source answers "what should I do now?", and the answer is
// almost always unfinished work — exactly what a publication world like
// `published` excludes. A source that inherited the reader's world would
// therefore be reliably empty in the world most people browse, which is why
// the query world is operator-declared per source (`source_world:`) and an
// absent one means the DEFAULT world rather than "whatever the caller asked
// for".
//
// That is also the security-relevant half. The request's `?world=` is
// caller-supplied; a source world is operator-supplied. Letting the URL
// parameter widen or redirect a source's query would let a caller aim an
// operator's rule at a world the operator never named. The caller's world
// governs DISPLAY only (dataentryconfig.NextActionSource.VisibleWorlds,
// applied in the handler); it is dropped here, so the two axes cannot be
// conflated by a request.
//
// # The ACL check is not optional, and it is a SKIP
//
// A declared source world is config, so it is not itself an authorization: the
// grant is re-checked per principal, exactly as resolveWorld does for `?world=`.
// A principal with no read grant contributes no candidates from this source.
//
// SKIP rather than error, deliberately. This is an advisory surface that sits
// on other pages, so failing the whole resolve would let one misconfigured
// source blank the suggestion box for everyone it does not apply to — and the
// principal who cannot read `editorial` is a perfectly ordinary user, not an
// error condition. Skipping degrades to "no suggestion from this source",
// which is a state the engine already handles on every quiet day.
//
// Note this is belt-and-braces rather than the boundary: candidates flow
// through the caller's read gate regardless, so a missing grant here could at
// most have produced rows the gate would then have withheld. Checking anyway
// keeps the world grant meaningful as a grant and matches the request path.
//
// An infrastructure failure from the gate is NOT a denial and propagates —
// rendering an outage as a quiet suggestion box hides it with no operator
// signal (the RR-4TFZNL shape).
//
// Returns ctx unchanged for the default world, so a project that declares no
// worlds takes exactly the path it did before this key existed.
//
// A free function taking the lookup rather than an App method, for the same
// reason resolveWorld is one: it needs exactly one collaborator, and App is at
// its plimsoll method cap because it has accreted for years — adding to it is
// the habit that got it there.
func nextActionSourceWorld(
	ctx context.Context, lookup WorldLookup, src dataentryconfig.NextActionSource,
) (context.Context, bool, error) {
	name := src.SourceWorld
	if name == "" || name == defaultWorldName {
		// The default world is today's graph and needs no grant beyond the
		// per-entity gates that already run. Bind the zero handle explicitly
		// so a request that arrived with `?world=published` cannot leak its
		// scope into a source that never named one.
		return withWorld(ctx, worldHandle{}), true, nil
	}
	if lookup == nil {
		// Worlds were never wired, so no name can resolve. Skip rather than
		// silently querying the default world under a source that asked for
		// something else.
		return ctx, false, nil
	}
	scope, known := lookup.Lookup(name)
	if !known {
		// Rejected at config load (validateNextActionWorlds). Reaching here
		// means config and metamodel disagree at runtime; skip rather than
		// widening to the default world.
		return ctx, false, nil
	}
	permitted, err := readGateFromContext(ctx).PermitsWorld(ctx, name)
	if err != nil {
		return ctx, false, fmt.Errorf("next-action source world %q: %w", name, err)
	}
	if !permitted {
		return ctx, false, nil
	}
	return withWorld(ctx, worldHandle{name: name, scope: scope}), true, nil
}

// nextActionOptions resolves a pick_one option list through the SAME
// ACL-gated path as candidates, so an option can never name an entity the
// caller may not read.
//
// Labels come from the entity's display title, which the read gate has
// already approved — this is why the resolution belongs here and not in the
// SPA: a client-side fetch would be a second read surface to gate.
func (a *App) nextActionOptions() nextaction.OptionFunc {
	return func(
		ctx context.Context, src dataentryconfig.NextActionSource, query string, limit int,
	) ([]nextaction.PickOption, error) {
		// Options are scoped to the SOURCE's world, exactly like its
		// candidates. An option list resolved in a different world than the
		// suggestion it belongs to would offer the user entities that do not
		// exist from where the suggestion was computed.
		ctx, ok, err := nextActionSourceWorld(ctx, a.worlds, src)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		entities, err := a.queries.executeQuery(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("next-action pick_one query %q: %w", query, err)
		}
		if len(entities) > limit {
			entities = entities[:limit]
		}
		out := make([]nextaction.PickOption, 0, len(entities))
		s := a.State()
		for _, e := range entities {
			// Redact BEFORE safeDisplayTitle: its guard is presence-based
			// ("is the display property missing?"), so against an unredacted
			// entity every property is present and it happily returns the
			// hidden value.
			red := a.redactedForSuggestion(ctx, e)
			out = append(out, nextaction.PickOption{
				EntityID: e.ID,
				// safeDisplayTitle, not the raw DisplayTitle: a redacted
				// display property would otherwise render a PARTIAL title,
				// leaking the readable half and confirming a hidden half
				// exists (the BUG-R9EHKV leak class).
				// One snapshot for the whole loop (captured above): two
				// State() loads can observe different metamodels if a reload
				// lands mid-resolve.
				Label: safeDisplayTitle(s.Meta, red),
			})
		}
		return out, nil
	}
}

// queryCandidates runs a source's query through the ACL-gated pipeline.
//
// Candidates are FIELD-REDACTED here, not merely row-gated. executeQuery
// returns raw entities — every other consumer redacts at serialization (see
// forWireRelated / stripHiddenProperties), but a suggestion never passes
// through that seam: its message interpolates `{property}` straight into
// text. Without this an operator writing `suggest: "{title} needs a look"` on
// a type whose title is hidden by `visible:` would put the hidden value on
// the wire — the BUG-R9EHKV leak class, through a new door.
func (a *App) queryCandidates(ctx context.Context, query string) ([]nextaction.Candidate, error) {
	entities, err := a.queries.executeQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("next-action query %q: %w", query, err)
	}
	out := make([]nextaction.Candidate, 0, len(entities))
	for _, e := range entities {
		out = append(out, nextaction.Candidate{Entity: a.redactedForSuggestion(ctx, e)})
	}
	return out, nil
}

// redactedForSuggestion returns a copy of e carrying only the properties this
// principal may see.
//
// A COPY, never a mutation: the entity comes from the store's iterator and may
// be shared with a cache, so stripping in place would redact it for everyone.
func (a *App) redactedForSuggestion(ctx context.Context, e *entityPkg.Entity) *entityPkg.Entity {
	clone := *e
	clone.Properties = a.affordances.copyVisibleProperties(ctx, e)
	return &clone
}

// countCandidates answers a `<entity_type> == 0` source: it yields exactly one
// entity-less candidate when the type is empty, and none otherwise.
//
// GATED BY DEFAULT. The count runs through the same read path as every other
// source, so it counts what THIS principal can see: "do I have any clients?",
// not "does anyone?". An ungated count would disclose that entities of a type
// exist to a principal permitted to read none of them, and entity existence is
// the strongest secret rela's read model keeps (a hidden entity is
// nonexistent, indistinguishable from a real 404).
//
// `ungated` (config: count_ungated) opts out for a genuinely operator-level
// question — "has this deployment been set up at all?" — and is never
// inferred. The safe default's failure mode is a principal who sees no clients
// being repeatedly offered "add a client": mild, visible, and fixable in
// config. The unsafe default's failure mode is a silent disclosure nobody
// notices. Prefer the loud, fixable error.
func (a *App) countCandidates(
	ctx context.Context, count string, ungated bool,
) ([]nextaction.Candidate, error) {
	entityType, ok := parseCountZero(count)
	if !ok {
		// Validated at config load; reaching here means the config was built
		// in code. Yield nothing rather than failing the whole page.
		return nil, nil
	}

	empty, err := a.countIsZero(ctx, entityType, ungated)
	if err != nil {
		return nil, err
	}
	if !empty {
		return nil, nil
	}
	// One candidate with no entity — the engine keys it on the source alone.
	return []nextaction.Candidate{{}}, nil
}

// countIsZero reports whether the caller sees no entities of entityType.
func (a *App) countIsZero(ctx context.Context, entityType string, ungated bool) (bool, error) {
	if ungated {
		_, total, err := a.Services().Store.GraphCount(ctx, store.GraphQuery{EntityType: entityType})
		if err != nil {
			return false, fmt.Errorf("next-action count for %q: %w", entityType, err)
		}
		return total == 0, nil
	}

	// Gated: route through the same read gate the list pipeline uses, so a
	// hidden entity does not count. scopedSortedEntities resolves the ACL
	// verdict FIRST and returns empty on DenyAll, which is exactly the
	// answer we want — a principal denied the type sees "none", and the
	// first-run hint fires for them.
	entities, err := a.scopedSortedEntities(ctx, entityType, nil)
	if err != nil {
		return false, fmt.Errorf("next-action count for %q: %w", entityType, err)
	}
	return len(entities) == 0, nil
}

// parseCountZero extracts the entity type from the only supported aggregate
// form, "<entity_type> == 0". Mirrors validateNextActionCount so the engine
// and the validator agree on what is accepted.
func parseCountZero(count string) (string, bool) {
	fields := strings.Fields(count)
	if len(fields) != 3 || fields[1] != "==" || fields[2] != "0" {
		return "", false
	}
	return fields[0], true
}

// SetUserState replaces the next-action per-user state backend.
//
// Defaults to an in-memory store (see NewApp), which is correct for a
// single-process deployment: this state is disposable, so losing it on
// restart costs a user one repeated suggestion. A deployment that wants
// snoozes to survive a restart, or to be shared across processes, injects a
// persistent backend here.
//
// Rejects nil rather than quietly disabling the feature: a silently absent
// store would make the app stop honoring snoozes and mutes, which users
// experience as the system ignoring them — the deferred-failure symptom the
// project's constructor rule exists to prevent.
func (a *App) SetUserState(s userstate.Store) error {
	if s == nil {
		return errors.New("dataentry.SetUserState: store must be non-nil")
	}
	a.userState = s
	return nil
}

// SetNextActionMatchers injects the predicate compiler backing a source's
// `condition:`.
//
// Separate from NewApp for the same reason as [App.SetUserState]: the compiler
// lives above this package, so the composition root supplies it rather than
// this package importing it. Rejects nil for the same reason too — a silently
// absent compiler would leave every condition unevaluated, showing suggestions
// for entities the operator explicitly excluded.
func (a *App) SetNextActionMatchers(fn NextActionMatcherFunc) error {
	if fn == nil {
		return errors.New("dataentry.SetNextActionMatchers: func must be non-nil")
	}
	a.nextActionMatchers = fn
	return nil
}

// NextActionMatcherFunc compiles the `condition:` of every configured source
// against the current metamodel, returning a per-source lookup plus one
// message per problem.
//
// The consumer-side seam for the predicate compiler: this package must not
// import it (arch-lint keeps the condition/policy engine above the data-entry
// app), so the composition root supplies an implementation.
type NextActionMatcherFunc func(
	cfg *dataentryconfig.Config, meta *metamodel.Metamodel,
) (func(sourceID string) (nextaction.Matcher, bool), []string)
