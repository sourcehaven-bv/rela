package sync

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// splice applies a redacted fetch onto the replica's own local record without
// erasing fields the primary withheld — the core of TKT-8P1TM7's no-data-loss
// guarantee ("sync is a fancy browser").
//
// The primary's /api/v1 read returns VISIBLE property values in `properties`
// and the NAMES it withheld in `_redacted` (DEC-T0XIWQ). That lets the replica
// tell three states apart for every property, so a redacted read can drive a
// faithful replica:
//
//	present in properties        → visible current value → upsert it
//	named in _redacted           → hidden from me        → leave my local copy
//	in NEITHER                   → deleted on the primary → unset it locally
//
// Hidden fields are out of scope for replication: the replica keeps whatever it
// last stored (it is not entitled to the current value), and never erases it.
//
// This is safe against the "never redact a read that feeds a write" trap
// because the merge target is the replica's OWN raw local record (read
// unredacted from the local fsstore — Mode A has no local ACL), not the
// redacted wire body. The splice is pure correctness here, not a security
// boundary. We then land the merged whole record through the sanctioned
// id-preserving, automation-suppressed ApplyEntity/ApplyRelation.

// spliceEntity merges a redacted entity fetch onto the raw local entity (or
// creates it if absent) and applies the result. It returns the entity as
// applied so the caller can canonically hash it for the index Local token.
func (e *Engine) spliceEntity(ctx context.Context, key string, fe *FetchedEntity) (*entity.Entity, error) {
	prior, err := e.store.GetEntity(ctx, key)
	switch {
	case err == nil:
		// exists locally → splice onto the raw record
	case errors.Is(err, store.ErrNotFound):
		prior = nil // first landing → create from the (visible) fetched body
	default:
		return nil, fmt.Errorf("read local entity %s: %w", key, err)
	}

	merged := mergeProperties(priorProps(prior), fe.Body.Properties, fe.Body.Redacted)

	ent := &entity.Entity{
		ID:         nonEmpty(fe.Body.ID, key),
		Type:       fe.Body.Type,
		Properties: merged,
		Content:    fe.Body.Content,
	}
	if _, err := e.applier.ApplyEntity(ctx, ent); err != nil {
		return nil, fmt.Errorf("apply entity %s: %w", key, err)
	}
	return ent, nil
}

// spliceRelation is the relation analog of spliceEntity, merging redacted
// relation meta onto the raw local relation.
func (e *Engine) spliceRelation(
	ctx context.Context, from, relType, to string, fr *FetchedRelation,
) (*entity.Relation, error) {
	prior, err := e.store.GetRelation(ctx, from, relType, to)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrNotFound):
		prior = nil
	default:
		return nil, fmt.Errorf("read local relation %s/%s/%s: %w", from, relType, to, err)
	}

	merged := mergeProperties(priorRelProps(prior), fr.Body.Properties, fr.Body.Redacted)

	rel := &entity.Relation{
		From: from, Type: relType, To: to,
		Properties: merged,
		Content:    fr.Body.Content,
	}
	if _, err := e.applier.ApplyRelation(ctx, rel); err != nil {
		return nil, fmt.Errorf("apply relation %s/%s/%s: %w", from, relType, to, err)
	}
	return rel, nil
}

// mergeProperties implements the three-state splice. prior is the replica's own
// raw local property map (may be nil for a first landing); visible is the
// primary's redacted `properties`; redacted names the withheld properties (nil
// when the response carried no field affordances — treated as "nothing known to
// be hidden", i.e. absence means deleted).
//
// The result is a fresh map: every visible field upserted; every prior field
// the primary neither sent nor flagged as hidden dropped (a genuine delete);
// every hidden (redacted) prior field preserved untouched.
func mergeProperties(prior, visible map[string]any, redacted *[]string) map[string]any {
	hidden := map[string]struct{}{}
	if redacted != nil {
		for _, name := range *redacted {
			hidden[name] = struct{}{}
		}
	}

	out := make(map[string]any, len(prior)+len(visible))
	// Keep prior values ONLY for fields the primary flagged as hidden — those are
	// the fields the replica is not entitled to and must not touch. Every other
	// prior field is governed by the fetched body: present → overwritten below,
	// absent → intentionally dropped (delete).
	for k, v := range prior {
		if _, isHidden := hidden[k]; isHidden {
			out[k] = v
		}
	}
	// Upsert every visible field the primary sent.
	maps.Copy(out, visible)
	return out
}

// priorProps returns an entity's property map, or nil when the entity is absent.
func priorProps(e *entity.Entity) map[string]any {
	if e == nil {
		return nil
	}
	return e.Properties
}

// priorRelProps returns a relation's property map, or nil when absent.
func priorRelProps(r *entity.Relation) map[string]any {
	if r == nil {
		return nil
	}
	return r.Properties
}

// nonEmpty returns a if non-empty, else fallback.
func nonEmpty(a, fallback string) string {
	if a == "" {
		return fallback
	}
	return a
}
