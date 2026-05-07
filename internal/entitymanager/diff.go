package entitymanager

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

// entityReader is the read surface the diff/propagation/validation
// helpers need from the underlying graph state. *workspace.Tx
// satisfies it; tests can satisfy it with a fake to drive the
// helpers without a workspace.
type entityReader interface {
	GetEntity(id string) (*model.Entity, bool)
	GetRelation(from, relType, to string) (*model.Relation, bool)
	OutgoingEdges(id string) []*model.Relation
}

// relationKey is a (from, type, to) triple identifying one edge.
type relationKey struct {
	from    string
	relType string
	to      string
}

// relationDiff is the staged outcome of computing a PATCH's relation
// changes against the current graph state.
type relationDiff struct {
	// adds is relations to write (covers add and update-meta).
	adds []*model.Relation
	// removes is the keys of edges to delete.
	removes []relationKey
	// counterparties is the set of entity IDs whose outgoing
	// relations were touched via symmetric/inverse propagation.
	counterparties map[string]struct{}
}

// computeDiff walks the request's relations, compares against the
// current graph state, validates each entry, and populates diff.adds /
// diff.removes for primary (non-propagated) changes.
//
// Validation runs BEFORE the no-op suppression check so a
// schema-violating value can never sneak through as "no-op".
func computeDiff(r entityReader, meta *metamodel.Metamodel, entityID, fromType string,
	relations map[string]RelationDesiredState, diff *relationDiff,
) error {
	for relType, desired := range relations {
		if err := computeDiffForType(r, meta, entityID, fromType, relType, desired.Edges, diff); err != nil {
			return err
		}
	}
	return nil
}

// computeDiffForType is computeDiff scoped to one relation type.
func computeDiffForType(r entityReader, meta *metamodel.Metamodel,
	entityID, fromType, relType string, edges []RelationRef, diff *relationDiff,
) error {
	relDef, ok := meta.GetRelationDef(relType)
	if !ok {
		return validationErrorf("relations[%q]: unknown relation type", relType)
	}

	if !relDef.Content {
		for i, ref := range edges {
			if ref.Content != nil {
				return validationErrorf(
					"/relations/%q/data/%d/content: relation type %q does not support content body",
					relType, i, relType)
			}
		}
	}

	desiredByID := make(map[string]RelationRef, len(edges))
	for _, ref := range edges {
		desiredByID[ref.ID] = ref
	}

	currentByTarget := snapshotOutgoing(r, entityID, relType)

	for _, ref := range edges {
		if err := stageDesiredEdge(
			r, meta, relDef, entityID, fromType, relType, ref, currentByTarget, diff,
		); err != nil {
			return err
		}
	}

	for targetID := range currentByTarget {
		if _, kept := desiredByID[targetID]; kept {
			continue
		}
		diff.removes = append(diff.removes, relationKey{from: entityID, relType: relType, to: targetID})
	}
	return nil
}

// snapshotOutgoing builds a map of the live outgoing edges of the given
// type, indexed by target ID. Duplicates indicate graph corruption; we
// keep the last one for diff purposes but log a warning.
func snapshotOutgoing(r entityReader, entityID, relType string) map[string]*model.Relation {
	out := map[string]*model.Relation{}
	for _, edge := range r.OutgoingEdges(entityID) {
		if edge.Type != relType {
			continue
		}
		if _, dup := out[edge.To]; dup {
			slog.Warn("duplicate edge in graph (last-writer-wins for diff)",
				"from", entityID, "type", relType, "to", edge.To)
		}
		out[edge.To] = edge
	}
	return out
}

// stageDesiredEdge validates one desired edge, computes its final
// (post-merge) state, and appends it to diff.adds unless the final
// state matches the existing edge byte-for-byte.
func stageDesiredEdge(r entityReader, meta *metamodel.Metamodel, relDef *metamodel.RelationDef,
	entityID, fromType, relType string, ref RelationRef,
	currentByTarget map[string]*model.Relation, diff *relationDiff,
) error {
	targetEntity, exists := r.GetEntity(ref.ID)
	if !exists {
		return validationErrorf(
			"/relations/%q/data/*/id: target %q does not exist", relType, ref.ID)
	}
	if targetEntity.Type != ref.Type {
		return validationErrorf(
			"/relations/%q/data/*/type: expected %q, got %q for target %q",
			relType, targetEntity.Type, ref.Type, ref.ID)
	}
	if err := meta.ValidateRelation(relType, fromType, targetEntity.Type); err != nil {
		return validationErrorf("relations[%q]: %v", relType, err)
	}
	if err := validateMetaKeys(relDef, ref, relType); err != nil {
		return err
	}

	finalProps, finalContent := mergeEdgeFields(currentByTarget[ref.ID], ref)

	rel := model.NewRelation(entityID, relType, ref.ID)
	rel.Properties = finalProps
	rel.Content = finalContent
	if errs := meta.ValidateRelationProperties(rel); len(errs) > 0 {
		return validationErrorf(
			"/relations/%q/data/*/meta: %s", relType, errs[0].Message)
	}

	if existing, ok := currentByTarget[ref.ID]; ok {
		if model.PropertyMapsEqual(existing.Properties, finalProps) && existing.Content == finalContent {
			return nil
		}
	}

	diff.adds = append(diff.adds, rel)
	return nil
}

// validateMetaKeys checks ref.Meta and ref.MetaUnset against the
// relation type's closed property schema.
func validateMetaKeys(relDef *metamodel.RelationDef, ref RelationRef, relType string) error {
	for k := range ref.Meta {
		if _, known := relDef.Properties[k]; !known {
			return validationErrorf(
				"/relations/%q/data/*/meta/%q: unknown property for relation type %q",
				relType, k, relType)
		}
	}
	for _, k := range ref.MetaUnset {
		if _, known := relDef.Properties[k]; !known {
			return validationErrorf(
				"/relations/%q/data/*/meta_unset: unknown property %q for relation type %q",
				relType, k, relType)
		}
	}
	return nil
}

// mergeEdgeFields returns the post-merge properties + content for an
// edge, given the existing edge (or nil) and the desired ref.
func mergeEdgeFields(existing *model.Relation, ref RelationRef) (props map[string]interface{}, content string) {
	finalProps := map[string]interface{}{}
	finalContent := ""
	if existing != nil {
		for k, v := range existing.Properties {
			finalProps[k] = v
		}
		finalContent = existing.Content
	}
	for k, v := range ref.Meta {
		finalProps[k] = v
	}
	for _, k := range ref.MetaUnset {
		delete(finalProps, k)
	}
	if ref.Content != nil {
		finalContent = *ref.Content
	}
	return finalProps, finalContent
}

// propagateRelations walks the primary diff and stages symmetric/inverse
// counterparty edges. Adds/removes are gated on no-op suppression
// against the current graph state. Properties are deep-copied to avoid
// aliasing. Self-loops are skipped on both symmetric and inverse.
//
// Modifies diff in place: extends diff.adds, diff.removes, and
// diff.counterparties.
func propagateRelations(r entityReader, meta *metamodel.Metamodel, diff *relationDiff) {
	primaryAdds := append([]*model.Relation(nil), diff.adds...)
	primaryRemoves := append([]relationKey(nil), diff.removes...)

	for _, rel := range primaryAdds {
		propagateAdd(r, meta, rel, diff)
	}
	for _, rem := range primaryRemoves {
		propagateRemove(r, meta, rem, diff)
	}
}

// propagateAdd stages the symmetric and/or inverse counterpart edges
// for a primary add.
func propagateAdd(r entityReader, meta *metamodel.Metamodel, rel *model.Relation, diff *relationDiff) {
	def, ok := meta.GetRelationDef(rel.Type)
	if !ok {
		return
	}
	if def.Symmetric && rel.From != rel.To {
		back := model.NewRelation(rel.To, rel.Type, rel.From)
		back.Properties = cloneProps(rel.Properties)
		back.Content = rel.Content
		stageAdd(r, diff, back)
	}
	if def.Inverse != nil && def.Inverse.ID != "" && rel.From != rel.To {
		invDef, invOK := meta.GetRelationDef(def.Inverse.ID)
		back := model.NewRelation(rel.To, def.Inverse.ID, rel.From)
		back.Properties = cloneProps(rel.Properties)
		// Only carry content across when the inverse type declares
		// Content: true. Without this guard the back-edge silently
		// gets a body the inverse schema disallows;
		// validateStagedEdges would not catch this because the
		// relation-properties validator doesn't see the content field.
		if invOK && invDef.Content {
			back.Content = rel.Content
		}
		stageAdd(r, diff, back)
	}
}

// propagateRemove stages the symmetric and/or inverse counterpart edge
// removes for a primary remove.
func propagateRemove(r entityReader, meta *metamodel.Metamodel, rem relationKey, diff *relationDiff) {
	def, ok := meta.GetRelationDef(rem.relType)
	if !ok {
		return
	}
	if rem.from == rem.to {
		return
	}
	if def.Symmetric {
		stageRemove(r, diff, relationKey{from: rem.to, relType: rem.relType, to: rem.from})
	}
	if def.Inverse != nil && def.Inverse.ID != "" {
		stageRemove(r, diff, relationKey{from: rem.to, relType: def.Inverse.ID, to: rem.from})
	}
}

// stageAdd appends an edge to diff.adds unless an identical edge
// already exists in the graph (no-op suppression).
func stageAdd(r entityReader, diff *relationDiff, back *model.Relation) {
	if existing, ok := r.GetRelation(back.From, back.Type, back.To); ok {
		if model.PropertyMapsEqual(existing.Properties, back.Properties) && existing.Content == back.Content {
			return
		}
	}
	diff.adds = append(diff.adds, back)
	diff.counterparties[back.From] = struct{}{}
}

// stageRemove appends an edge key to diff.removes unless the edge
// doesn't exist in the graph (nothing to remove).
func stageRemove(r entityReader, diff *relationDiff, back relationKey) {
	if _, ok := r.GetRelation(back.from, back.relType, back.to); !ok {
		return
	}
	diff.removes = append(diff.removes, back)
	diff.counterparties[back.from] = struct{}{}
}

// validateStagedEdges runs full validation against every entry in the
// diff's adds — primary AND propagated. Catches non-existent
// counterparty target, type allowlist violations on the inverse side,
// and meta property type violations on the back-edge.
func validateStagedEdges(r entityReader, meta *metamodel.Metamodel, edges []*model.Relation) error {
	for _, rel := range edges {
		fromEnt, ok := r.GetEntity(rel.From)
		if !ok {
			return validationErrorf(
				"propagated edge %s -%s-> %s: source entity %q does not exist",
				rel.From, rel.Type, rel.To, rel.From)
		}
		toEnt, ok := r.GetEntity(rel.To)
		if !ok {
			return validationErrorf(
				"propagated edge %s -%s-> %s: target entity %q does not exist",
				rel.From, rel.Type, rel.To, rel.To)
		}
		if err := meta.ValidateRelation(rel.Type, fromEnt.Type, toEnt.Type); err != nil {
			return validationErrorf(
				"propagated edge %s -%s-> %s: %v", rel.From, rel.Type, rel.To, err)
		}
		if errs := meta.ValidateRelationProperties(rel); len(errs) > 0 {
			return validationErrorf(
				"propagated edge %s -%s-> %s: %s",
				rel.From, rel.Type, rel.To, errs[0].Message)
		}
	}
	return nil
}

// cloneProps returns a shallow copy of a relation's properties map.
// YAML scalar values are immutable in practice so a deeper copy is not
// needed.
func cloneProps(props map[string]interface{}) map[string]interface{} {
	if props == nil {
		return nil
	}
	out := make(map[string]interface{}, len(props))
	for k, v := range props {
		out[k] = v
	}
	return out
}

// entitiesEqual returns true if two entities have identical persisted
// state (properties + content). Used by no-op suppression.
func entitiesEqual(a, b *model.Entity) bool {
	if a.Content != b.Content {
		return false
	}
	return model.PropertyMapsEqual(a.Properties, b.Properties)
}
