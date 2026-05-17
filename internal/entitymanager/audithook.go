package entitymanager

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// recordEntityAudit emits one audit record for an entity create /
// update / delete success.
//
// Property *names* may appear in the summary; property *values* never
// do — defense against secrets accidentally stored in properties.
func (m *Manager) recordEntityAudit(ctx context.Context, op string, e *entity.Entity, summary string) {
	if e == nil {
		return
	}
	m.deps.Audit.Record(audit.Record{
		Time: time.Now().UTC(),
		Op:   op,
		Subject: &audit.Subject{
			Kind: "entity",
			Type: e.Type,
			ID:   e.ID,
		},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     summary,
	})
}

// recordRelationAudit emits one audit record for a relation create /
// update / delete success.
func (m *Manager) recordRelationAudit(ctx context.Context, op string, rel *entity.Relation, summary string) {
	if rel == nil {
		return
	}
	m.deps.Audit.Record(audit.Record{
		Time: time.Now().UTC(),
		Op:   op,
		Subject: &audit.Subject{
			Kind:         "relation",
			RelationType: rel.Type,
			FromID:       rel.From,
			ToID:         rel.To,
		},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     summary,
	})
}

// recordRenameAudit emits the rename record with Before / After
// populated and Subject empty. The schema asymmetry is intentional —
// rename is the only op where the identity changes, so it's the only
// op that needs two subjects.
func (m *Manager) recordRenameAudit(ctx context.Context, before, after *entity.Entity) {
	if before == nil || after == nil {
		return
	}
	m.deps.Audit.Record(audit.Record{
		Time: time.Now().UTC(),
		Op:   audit.OpRenameEntity,
		Before: &audit.Subject{
			Kind: "entity",
			Type: before.Type,
			ID:   before.ID,
		},
		After: &audit.Subject{
			Kind: "entity",
			Type: after.Type,
			ID:   after.ID,
		},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     "renamed",
	})
}

// updateEntitySummary builds an update-entity record's Summary
// from the old / new entity state. Returns "updated" if no
// property names changed (content-only edit or no diff at all);
// otherwise "updated: prop1,prop2,...".
func updateEntitySummary(oldE, newE *entity.Entity) string {
	if oldE == nil || newE == nil {
		return "updated"
	}
	names := changedPropertyNames(oldE.Properties, newE.Properties)
	if names == "" {
		return "updated"
	}
	return "updated: " + names
}

// updateRelationSummary builds an update-relation record's Summary
// from the pre- / post-update relation meta maps.
func updateRelationSummary(oldProps, newProps map[string]interface{}) string {
	names := changedPropertyNames(oldProps, newProps)
	if names == "" {
		return "updated"
	}
	return "updated: " + names
}

// cloneProperties returns a shallow copy of props. Used to snapshot
// pre-update state for change-summary computation.
func cloneProperties(props map[string]interface{}) map[string]interface{} {
	if props == nil {
		return nil
	}
	out := make(map[string]interface{}, len(props))
	for k, v := range props {
		out[k] = v
	}
	return out
}

// changedPropertyNames returns the sorted, comma-joined names of
// properties that changed between old and new. Used to construct the
// summary for update-entity / update-relation records. Property
// VALUES are never included — keys only.
//
// "Changed" means: the stringified values differ. fmt.Sprint
// stringification is good enough for change detection in summaries —
// we're not relying on this for correctness elsewhere.
func changedPropertyNames(oldProps, newProps map[string]interface{}) string {
	seen := make(map[string]bool, len(oldProps)+len(newProps))
	for k := range oldProps {
		seen[k] = true
	}
	for k := range newProps {
		seen[k] = true
	}
	var changed []string
	for k := range seen {
		if fmt.Sprint(oldProps[k]) != fmt.Sprint(newProps[k]) {
			changed = append(changed, k)
		}
	}
	sort.Strings(changed)
	return strings.Join(changed, ",")
}
