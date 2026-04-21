package dataentry

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// relationError is returned by reconcileOutgoingRelations to surface a
// per-edge failure with the relation type and target id attached. The
// handler layer uses errors.As to extract the structured fields for the
// problem-details response so the UI can attribute the failure to the
// specific chip that caused it.
type relationError struct {
	RelType string
	Target  string
	Op      string // "create" | "delete" | "validate"
	Reason  string // stable reason code
	Err     error  // underlying error, for Unwrap / logs
}

func (e *relationError) Error() string {
	if e.Target != "" {
		return fmt.Sprintf("relation %s %s %s: %s", e.Op, e.RelType, e.Target, e.Reason)
	}
	return fmt.Sprintf("relation %s %s: %s", e.Op, e.RelType, e.Reason)
}

func (e *relationError) Unwrap() error { return e.Err }

// reconcileOutgoingRelations brings the outgoing edges of entityID in line
// with desired: for each relation type present in desired, edges to targets
// not currently present are created and edges to targets no longer present
// are deleted. Relation types absent from desired are left untouched, so
// callers can reconcile a subset (e.g. only chip-picker relations, leaving
// card-widget relations managed via their own per-edge endpoints).
//
// A nil or empty desired map is a no-op. Before any writes, every relation
// type and target is validated against the metamodel: unknown relation
// types, source-type mismatches, and missing targets surface as typed
// *relationError with a stable Reason code rather than a raw Go string from
// the workspace layer.
func (a *App) reconcileOutgoingRelations(ctx context.Context, entityID string, desired map[string][]string) error {
	if len(desired) == 0 {
		return nil
	}

	meta := a.State().Meta
	entity, ok := a.getEntity(entityID)
	if !ok {
		return &relationError{Op: "validate", Reason: "source_not_found", Err: fmt.Errorf("entity %s not found", entityID)}
	}

	// Validate every relation type and target up-front so a typo or
	// unknown id is surfaced cleanly without touching the store.
	for relType, targets := range desired {
		relDef, ok := meta.Relations[relType]
		if !ok {
			return &relationError{RelType: relType, Op: "validate", Reason: "unknown_relation_type"}
		}
		if !containsString(relDef.From, entity.Type) {
			return &relationError{RelType: relType, Op: "validate", Reason: "source_type_not_allowed",
				Err: fmt.Errorf("relation %s does not accept source type %s", relType, entity.Type)}
		}
		for _, target := range targets {
			t, ok := a.getEntity(target)
			if !ok {
				return &relationError{RelType: relType, Target: target, Op: "validate", Reason: "target_not_found"}
			}
			if len(relDef.To) > 0 && !containsString(relDef.To, t.Type) {
				return &relationError{RelType: relType, Target: target, Op: "validate", Reason: "target_type_not_allowed",
					Err: fmt.Errorf("relation %s does not accept target type %s", relType, t.Type)}
			}
		}
	}

	current, err := a.outgoingRelationsCtx(ctx, entityID)
	if err != nil {
		return fmt.Errorf("list current relations: %w", err)
	}

	currentByType := map[string]map[string]bool{}
	for _, r := range current {
		if _, ok := desired[r.Type]; !ok {
			continue
		}
		m, ok := currentByType[r.Type]
		if !ok {
			m = map[string]bool{}
			currentByType[r.Type] = m
		}
		m[r.To] = true
	}

	for relType, targets := range desired {
		wanted := make(map[string]bool, len(targets))
		for _, id := range targets {
			// Duplicates in the caller-supplied list collapse to the
			// set semantic the picker already uses.
			wanted[id] = true
		}

		// Add before remove: we rely on the store not enforcing outgoing
		// cardinality on write (it doesn't today; cardinality is an
		// analyze-time check). If that ever changes, reconcile must run
		// as an atomic batch — see follow-up RR-IXQ5Q and architect C2.
		for id := range wanted {
			if currentByType[relType][id] {
				continue
			}
			if _, err := a.entityManager.CreateRelation(ctx, entityID, relType, id, entitymanager.RelationOptions{}); err != nil {
				return &relationError{RelType: relType, Target: id, Op: "create", Reason: "create_failed", Err: err}
			}
		}
		for id := range currentByType[relType] {
			if wanted[id] {
				continue
			}
			if err := a.entityManager.DeleteRelation(ctx, entityID, relType, id); err != nil {
				return &relationError{RelType: relType, Target: id, Op: "delete", Reason: "delete_failed", Err: err}
			}
		}
	}

	return nil
}

// reconcileDetail formats a reconcile error for the problem-details
// response. If the error is a *relationError the output is a stable
// "relation=<t> target=<to> op=<op> reason=<reason>" string so the
// frontend can parse it; otherwise the raw error string is passed through.
func reconcileDetail(err error) string {
	var rerr *relationError
	if errors.As(err, &rerr) {
		base := fmt.Sprintf("relation=%s op=%s reason=%s", rerr.RelType, rerr.Op, rerr.Reason)
		if rerr.Target != "" {
			base += " target=" + rerr.Target
		}
		if rerr.Err != nil {
			base += fmt.Sprintf(" cause=%q", rerr.Err.Error())
		}
		return base
	}
	return err.Error()
}
