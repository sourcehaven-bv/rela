package dataentry

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
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
//   - Query  -> [App.executeQuery], the same helper /_search and scope
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
		switch {
		case src.Count != "":
			return a.countCandidates(ctx, src.Count)
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

// queryCandidates runs a source's query through the ACL-gated pipeline.
func (a *App) queryCandidates(ctx context.Context, query string) ([]nextaction.Candidate, error) {
	entities, err := a.executeQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("next-action query %q: %w", query, err)
	}
	out := make([]nextaction.Candidate, 0, len(entities))
	for _, e := range entities {
		out = append(out, nextaction.Candidate{Entity: e})
	}
	return out, nil
}

// countCandidates answers a `<entity_type> == 0` source: it yields exactly one
// entity-less candidate when the type is empty, and none otherwise.
//
// The count is deliberately UNGATED (store-level, not read-gated). "Does any
// client exist?" is a question about whether the operator has begun using the
// app, not about whether this principal may read any particular client — and
// gating it would make the first-run hint reappear for a principal who simply
// cannot see the clients that do exist, which is worse than useless.
//
// This is safe because a count source's suggestion mentions NO entity: it
// interpolates nothing, exposes no id, and its message is operator-authored
// static text. The only bit that reaches the user is "this type is empty",
// which the metamodel already tells them exists.
func (a *App) countCandidates(ctx context.Context, count string) ([]nextaction.Candidate, error) {
	entityType, ok := parseCountZero(count)
	if !ok {
		// Validated at config load; reaching here means the config was built
		// in code. Yield nothing rather than failing the whole page.
		return nil, nil
	}

	svc := a.Services()
	_, total, err := svc.Store.GraphCount(ctx, store.GraphQuery{EntityType: entityType})
	if err != nil {
		return nil, fmt.Errorf("next-action count for %q: %w", entityType, err)
	}
	if total != 0 {
		return nil, nil
	}
	// One candidate with no entity — the engine keys it on the source alone.
	return []nextaction.Candidate{{}}, nil
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
