package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// conversionRun is one Convert call. apply runs inside the Tx and notes how
// far it got; record writes the audit trail for what reached the store.
type conversionRun struct {
	c   *Converter
	req ConversionRequest
	// checked is the plan whose inputs passed requireReadableInputs.
	checked *conversionPlan

	plan  *conversionPlan
	newID string
	// renamed is set once the family carries newID in the store.
	renamed bool
	// removed and created are the relation writes that reached the store.
	removed, created []*entity.Relation
	visibleDrops     int
}

// apply plans the conversion from what view holds, runs every check, and
// only then writes: rename, replace the rows, rewrite the relations.
func (r *conversionRun) apply(ctx context.Context, view store.Store) error {
	m := r.c.m
	p, err := r.c.plan(ctx, view, r.req)
	if err != nil {
		return err
	}
	r.plan = p
	if checkErr := r.requireInputsUnchanged(p); checkErr != nil {
		return checkErr
	}
	if checkErr := p.requireCommittable(r.req); checkErr != nil {
		return checkErr
	}
	toDef, _ := m.deps.Meta.GetEntityDef(p.def.To)
	if checkErr := rejectUnknownTargetProperties(r.req.Properties, *toDef); checkErr != nil {
		return checkErr
	}
	newID, err := conversionTargetID(ctx, m, view, p.def.To, *toDef, r.req)
	if err != nil {
		return err
	}
	targets, err := r.buildTargets(ctx, view, newID)
	if err != nil {
		return err
	}
	problems, err := p.capacityProblems(ctx, view, m.deps.Meta)
	if err != nil {
		return err
	}
	if texts := relationProblemTexts(problems); len(texts) > 0 {
		return fmt.Errorf("%w: %v", ErrConversionInvalid, texts)
	}
	if checkErr := r.authorizeRemovals(ctx, view); checkErr != nil {
		return checkErr
	}
	replacer, ok := view.(store.FamilyReplacer)
	if !ok {
		return errors.New("entitymanager: store cannot replace entity families")
	}
	replacements, err := conversionReplacements(newID, p.family, targets, p.faceMap)
	if err != nil {
		return err
	}

	if _, err := view.RenameFamily(ctx, r.req.SourceID, newID); err != nil {
		return translateRenameErr(err, r.req.SourceID, newID)
	}
	r.newID, r.renamed = newID, true
	if err := replacer.ReplaceEntityFamily(ctx, newID, replacements); err != nil {
		return err
	}
	return r.rewriteRelations(ctx, view)
}

// conversionTargetID returns the converted entity's id: the caller's for a
// manual-id type, otherwise a generated one. It reads view, so the id is
// checked against the same state the rename writes to.
func conversionTargetID(
	ctx context.Context, m *Manager, view store.Store, toType string, target metamodel.EntityDef, req ConversionRequest,
) (string, error) {
	newID := req.TargetID
	if target.IsManualID() {
		if newID == "" {
			return "", fmt.Errorf("%w: target id is required for manual-id type", ErrConversionInvalid)
		}
		if err := entity.ValidateID(newID); err != nil {
			return "", fmt.Errorf("%w: %w", ErrConversionInvalid, err)
		}
	} else {
		if newID != "" {
			return "", fmt.Errorf("%w: target id is generated for this type", ErrConversionInvalid)
		}
		deps := m.deps
		deps.Store = view
		var err error
		if newID, err = generateID(ctx, deps, toType, "", 1); err != nil {
			return "", err
		}
	}
	if req.SourceID == newID {
		return "", fmt.Errorf("%w: target id must differ from source id", ErrConversionInvalid)
	}
	taken, err := idTakenByOther(ctx, view, newID, req.SourceID)
	if err != nil {
		return "", err
	}
	if taken {
		return "", fmt.Errorf("%w: %s", ErrEntityAlreadyExists, newID)
	}
	return newID, nil
}

// buildTargets returns the target row of every source row, checked as a
// create would be: the field gate on the caller's values, then computed
// fields, validation and `unique:`.
func (r *conversionRun) buildTargets(ctx context.Context, view store.Store, newID string) ([]*entity.Entity, error) {
	m, p, req := r.c.m, r.plan, r.req
	d := p.def
	targets := make([]*entity.Entity, 0, len(p.family))
	for _, source := range p.family {
		face, exists := p.faceMap[source.Face]
		if !exists {
			return nil, fmt.Errorf("%w: no destination face for %q", ErrConversionInvalid, source.Face)
		}
		props := conversionPropertyValues(d, source, req.Properties, req.SourceFace)
		if err := rejectComputedPresent(m.deps, d.To, props); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrConversionInvalid, err)
		}
		content := source.Content
		if d.Content != "" {
			content = conversionContentValue(d.Content, source)
		}
		supplied := source.Face.String() == req.SourceFace
		if req.Content != nil && supplied {
			content = *req.Content
		}
		target := &entity.Entity{ID: newID, Face: face, Type: d.To, Properties: props, Content: content}
		// The caller authors only the values it supplied; the mapped values
		// are the operator's, as automation output is for a create.
		if supplied && len(req.Properties) > 0 && fieldGated(m) {
			if err := checkCreateFieldWrite(ctx, m, target, req.Properties); err != nil {
				return nil, err
			}
		}
		before := &entity.Entity{Type: d.To, Properties: map[string]any{}}
		if err := rejectExternalRefChanges(m.deps.Meta, before, target, false); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrConversionInvalid, err)
		}
		if err := m.deps.Computed.Evaluate(ctx, target); err != nil {
			return nil, err
		}
		validation := metamodel.NewValidator(m.deps.Meta).ValidateEntity(target.ID, target.Type, target.Properties)
		if hard, _ := partitionValidationErrors(validation); len(hard) > 0 {
			return nil, newValidationError(hard)
		}
		if err := checkUniqueProperties(ctx, m.deps.Meta, view, target, r.req.SourceID); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

// authorizeRemovals authorizes deleting every relation the conversion
// removes, as a cascade delete authorizes its edges. A replacement edge needs
// no grant of its own: the convert grant covers it, as the rename grant
// covers the edges a rename re-keys.
func (r *conversionRun) authorizeRemovals(ctx context.Context, view store.Store) error {
	id := r.req.SourceID
	var incoming, outgoing []*entity.Relation
	for _, mv := range r.plan.moves {
		if mv.rel.From == id {
			outgoing = append(outgoing, mv.rel)
		} else {
			incoming = append(incoming, mv.rel)
		}
	}
	if len(incoming)+len(outgoing) == 0 {
		return nil
	}
	return r.c.m.authorizeCascadeRelations(ctx, view, id, r.plan.family, incoming, outgoing)
}

// rewriteRelations deletes each removed relation under its new id and
// creates its replacement. The rename has already re-keyed every edge.
func (r *conversionRun) rewriteRelations(ctx context.Context, view store.Store) error {
	oldID, newID := r.req.SourceID, r.newID
	renamed := func(id string) string {
		if id == oldID {
			return newID
		}
		return id
	}
	for _, mv := range r.plan.moves {
		current := entity.RelationKey{
			From: renamed(mv.rel.From), FromFace: mv.rel.FromFace, Type: mv.rel.Type, To: renamed(mv.rel.To),
		}
		if err := view.DeleteRelation(ctx, current); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		r.removed = append(r.removed, mv.rel)
		if mv.drop {
			if mv.visible {
				r.visibleDrops++
			}
			continue
		}
		key := mv.key
		key.From, key.To = renamed(key.From), renamed(key.To)
		data := &store.RelationData{Properties: mv.rel.Properties, Content: mv.rel.Content}
		created, err := view.CreateRelation(ctx, key, data)
		if err != nil {
			return fmt.Errorf("convert relation %q: %w", mv.rel.Type, err)
		}
		r.created = append(r.created, created)
	}
	return nil
}

// conversionReplacements pairs each target row with the source face it
// replaces. It writes nothing, and [store.ValidateFamilyReplacement] checks
// the result before the rename, so a plan the store would refuse is refused
// while nothing has been written.
func conversionReplacements(
	newID string, current, targets []*entity.Entity, faceMap map[entity.Face]entity.Face,
) ([]store.FamilyReplacement, error) {
	replacements := make([]store.FamilyReplacement, 0, len(targets))
	for _, target := range targets {
		for _, source := range current {
			if faceMap[source.Face] == target.Face {
				replacements = append(replacements, store.FamilyReplacement{SourceFace: source.Face, Entity: target})
				break
			}
		}
	}
	faces := make([]entity.Face, 0, len(current))
	for _, source := range current {
		faces = append(faces, source.Face)
	}
	if _, err := store.ValidateFamilyReplacement(newID, faces, replacements); err != nil {
		if errors.Is(err, store.ErrInvalidFamilyReplacement) {
			return nil, fmt.Errorf("%w: %w", ErrConversionInvalid, err)
		}
		return nil, err
	}
	return replacements, nil
}

// record writes the audit trail of what apply put in the store: per row a
// convert-entity record and a rename version, the alias rewrite and face
// moves, and per relation an audit record and a version. It returns the
// converted rows, read back from the store.
//
// lineage holds the lineage ids of the removed relations, read before the Tx.
func (r *conversionRun) record(ctx context.Context, lineage map[entity.RelationKey]int64) ([]*entity.Entity, error) {
	m, p := r.c.m, r.plan
	oldID, newID := r.req.SourceID, r.newID
	rows, readErr := familyRows(ctx, m.deps.Store, newID)
	rn := familyRename{
		m: m, oldID: oldID, newID: newID,
		conversionName: r.req.Definition, conversionFrom: p.def.From,
		prevFaces: make(map[entity.Face]entity.Face, len(p.faceMap)),
	}
	for src, dst := range p.faceMap {
		rn.prevFaces[dst] = src
	}
	if m.deps.RelationVersionRecorder != nil {
		rn.preRenameRels = p.kept
	}
	rn.record(ctx, rows)
	for src, dst := range p.faceMap {
		notifyAliasesOfFaceMove(ctx, m.deps.AliasRewriter, p.def.To, newID, src, dst)
	}

	trigger := "convert-entity:" + oldID + "->" + newID
	for _, rel := range r.removed {
		m.recordRelationAudit(ctx, audit.OpDeleteRelation, rel, "removed by conversion to "+p.def.To)
		m.recordRelationVersion(ctx, store.VersionOpDelete, rel, lineage[rel.Identity()], "", "", trigger)
	}
	for _, rel := range r.created {
		m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created by conversion to "+p.def.To)
	}
	return rows, readErr
}

// recordConversionAudit records the conversion of one face row: after is the
// converted row, oldID and oldType what it was.
func recordConversionAudit(ctx context.Context, m *Manager, name, oldID, oldType string, after *entity.Entity) {
	summary := "converted from " + oldType + " by " + name
	if !after.Face.IsImplicit() {
		summary += " face " + after.Face.String()
	}
	m.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpConvertEntity,
		Before:      &audit.Subject{Kind: "entity", Type: oldType, ID: oldID},
		After:       &audit.Subject{Kind: "entity", Type: after.Type, ID: after.ID},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     summary,
	})
}
