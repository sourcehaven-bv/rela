package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func conversionTargetID(
	ctx context.Context,
	m *Manager,
	d metamodel.ConversionDef,
	target metamodel.EntityDef,
	req ConversionRequest,
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
		var err error
		newID, err = generateID(ctx, m.deps, d.To, "", 1)
		if err != nil {
			return "", err
		}
	}
	if req.SourceID == newID {
		return "", fmt.Errorf("%w: target id must differ from source id", ErrConversionInvalid)
	}
	taken, err := idTakenByOther(ctx, m.deps.Store, newID, req.SourceID)
	if err != nil {
		return "", err
	}
	if taken {
		return "", fmt.Errorf("%w: %s", ErrEntityAlreadyExists, newID)
	}
	return newID, nil
}

func buildConversionTargets(
	ctx context.Context,
	m *Manager,
	d metamodel.ConversionDef,
	req ConversionRequest,
	family []*entity.Entity,
	faceMap map[entity.Face]entity.Face,
	newID string,
) ([]*entity.Entity, error) {
	targets := make([]*entity.Entity, 0, len(family))
	toDef, _ := m.deps.Meta.GetEntityDef(d.To)
	for _, source := range family {
		face, exists := faceMap[source.Face]
		if !exists {
			return nil, fmt.Errorf("%w: no destination face for %q", ErrConversionInvalid, source.Face)
		}
		props := conversionPropertyValues(d, source, req.Properties)
		for key := range req.Properties {
			if _, known := toDef.Properties[key]; !known {
				return nil, fmt.Errorf("%w: unknown target property %q", ErrConversionInvalid, key)
			}
		}
		if err := rejectComputedPresent(m.deps, d.To, props); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrConversionInvalid, err)
		}
		content := source.Content
		if d.Content != "" {
			content = conversionContentValue(d.Content, source)
		}
		if req.Content != nil {
			content = *req.Content
		}
		target := &entity.Entity{ID: newID, Face: face, Type: d.To, Properties: props, Content: content}
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
		targets = append(targets, target)
	}
	return targets, nil
}

func validateConversionRelationMappings(
	m *Manager,
	d metamodel.ConversionDef,
	req ConversionRequest,
	rels []*entity.Relation,
) error {
	for _, rel := range rels {
		mapped, exists := d.Relations[rel.Type]
		if !exists {
			return fmt.Errorf("%w: relation %q needs an explicit mapping or drop", ErrConversionInvalid, rel.Type)
		}
		if mapped == "drop" {
			if !slices.Contains(req.DropRelations, rel.Type) {
				return fmt.Errorf("%w: confirm dropping relation %q", ErrConversionInvalid, rel.Type)
			}
			continue
		}
		targetRel, exists := m.deps.Meta.GetRelationDef(mapped)
		if !exists {
			return fmt.Errorf("%w: relation %q maps to unknown relation %q", ErrConversionInvalid, rel.Type, mapped)
		}
		if rel.From == req.SourceID && !conversionAllowsType(targetRel.From, d.To) {
			return fmt.Errorf("%w: mapped relation %q cannot originate at %q", ErrConversionInvalid, mapped, d.To)
		}
		if rel.To == req.SourceID && !conversionAllowsType(targetRel.To, d.To) {
			return fmt.Errorf("%w: mapped relation %q cannot target %q", ErrConversionInvalid, mapped, d.To)
		}
	}
	for _, relType := range req.DropRelations {
		if d.Relations[relType] != "drop" {
			return fmt.Errorf("%w: relation %q is not configured to drop", ErrConversionInvalid, relType)
		}
	}
	return nil
}

func prepareConversionRename(
	m *Manager,
	req ConversionRequest,
	d metamodel.ConversionDef,
	newID string,
	faceMap map[entity.Face]entity.Face,
	rels []*entity.Relation,
) familyRename {
	rn := familyRename{m: m, oldID: req.SourceID, newID: newID, authorized: familyAuthorization{}}
	rn.conversionFrom = d.From
	rn.prevFaces = make(map[entity.Face]entity.Face, len(faceMap))
	for src, dst := range faceMap {
		rn.prevFaces[dst] = src
	}
	for _, rel := range rels {
		if d.Relations[rel.Type] != rel.Type {
			continue
		}
		if rel.From == req.SourceID && faceMap[rel.FromFace] != rel.FromFace {
			continue
		}
		rn.preRenameRels = append(rn.preRenameRels, rel)
	}
	return rn
}

func convertEntityTransaction(
	ctx context.Context,
	m *Manager,
	tx store.Transactor,
	d metamodel.ConversionDef,
	req ConversionRequest,
	newID string,
	faceMap map[entity.Face]entity.Face,
	targets []*entity.Entity,
	rels []*entity.Relation,
) (*entity.Entity, int, error) {
	var converted *entity.Entity
	dropped := 0
	txErr := tx.Tx(ctx, func(view store.Store) error {
		var err error
		converted, dropped, err = applyConversionInView(ctx, m, view, d, req, newID, faceMap, targets, rels)
		return err
	})
	return converted, dropped, txErr
}

func applyConversionInView(
	ctx context.Context,
	m *Manager,
	view store.Store,
	d metamodel.ConversionDef,
	req ConversionRequest,
	newID string,
	faceMap map[entity.Face]entity.Face,
	targets []*entity.Entity,
	rels []*entity.Relation,
) (*entity.Entity, int, error) {
	txCtx := store.ContextInTx(ctx)
	current, readErr := familyRows(txCtx, view, req.SourceID)
	if readErr != nil {
		return nil, 0, readErr
	}
	if len(current) == 0 {
		return nil, 0, newEntityNotFound(req.SourceID)
	}
	for _, row := range current {
		if row.Type != d.From {
			return nil, 0, ErrConversionSourceMismatch
		}
	}
	currentFaceMap, missing, invalid := resolveConversionFaces(d, current, req.FaceMapping)
	if len(missing) > 0 || len(invalid) > 0 || !maps.Equal(currentFaceMap, faceMap) {
		return nil, 0, ErrConversionInvalid
	}
	if err := requireReadableRow(txCtx, faceGateOf(m), req.SourceID, current); err != nil {
		return nil, 0, err
	}
	request := acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewFamilySubject(d.From, req.SourceID)}
	if err := m.authorizeAndAudit(txCtx, request); err != nil {
		return nil, 0, err
	}
	if _, err := view.RenameFamily(txCtx, req.SourceID, newID); err != nil {
		return nil, 0, translateRenameErr(err, req.SourceID, newID)
	}
	convertedRows, replaceErr := replaceConversionRows(txCtx, view, newID, current, targets, faceMap)
	if replaceErr != nil {
		return nil, 0, replaceErr
	}
	converted := chooseConvertedFace(convertedRows, faceMap, req.SourceFace)
	dropped, relationErr := rewriteConversionRelations(txCtx, m, view, req.SourceID, newID, d, faceMap, rels)
	return converted, dropped, relationErr
}

func replaceConversionRows(
	ctx context.Context,
	view store.Store,
	newID string,
	current, targets []*entity.Entity,
	faceMap map[entity.Face]entity.Face,
) ([]*entity.Entity, error) {
	replacer, ok := view.(store.FamilyReplacer)
	if !ok {
		return nil, errors.New("entitymanager: store cannot replace entity families")
	}
	replacements := make([]store.FamilyReplacement, 0, len(targets))
	for _, target := range targets {
		for _, source := range current {
			if faceMap[source.Face] == target.Face {
				replacements = append(replacements, store.FamilyReplacement{SourceFace: source.Face, Entity: target})
				break
			}
		}
	}
	if err := replacer.ReplaceEntityFamily(ctx, newID, replacements); err != nil {
		return nil, err
	}
	return familyRows(ctx, view, newID)
}

func rewriteConversionRelations(
	ctx context.Context,
	m *Manager,
	view store.Store,
	oldID, newID string,
	d metamodel.ConversionDef,
	faceMap map[entity.Face]entity.Face,
	rels []*entity.Relation,
) (int, error) {
	leaving, pending, invalid := conversionRelationPlan(oldID, newID, rels, d.Relations, faceMap, m.deps.Meta, d.To)
	if len(invalid) > 0 {
		return 0, fmt.Errorf("%w: %v", ErrConversionInvalid, invalid)
	}
	for i, key := range pending {
		if err := checkRelationCapacity(ctx, view, m.deps.Meta, key, leaving, pending[:i]); err != nil {
			return 0, err
		}
	}
	dropped := 0
	for _, rel := range rels {
		didDrop, err := convertRelation(ctx, m, view, oldID, newID, d, faceMap, rel)
		if err != nil {
			return dropped, err
		}
		if didDrop {
			dropped++
		}
	}
	return dropped, nil
}

func convertRelation(
	ctx context.Context,
	m *Manager,
	view store.Store,
	oldID, newID string,
	d metamodel.ConversionDef,
	faceMap map[entity.Face]entity.Face,
	rel *entity.Relation,
) (bool, error) {
	mapped := d.Relations[rel.Type]
	from, to := rel.From, rel.To
	if from == oldID {
		from = newID
	}
	if to == oldID {
		to = newID
	}
	fromFace := rel.FromFace
	if rel.From == oldID {
		fromFace = faceMap[fromFace]
	}
	if mapped == rel.Type && fromFace == rel.FromFace {
		return false, nil
	}
	key := entity.RelationKey{From: from, FromFace: rel.FromFace, Type: rel.Type, To: to}
	if err := view.DeleteRelation(ctx, key); err != nil && !errors.Is(err, store.ErrNotFound) {
		return false, err
	}
	if mapped == "drop" {
		return true, nil
	}
	relationType, exists := m.deps.Meta.GetRelationDef(mapped)
	if !exists {
		return false, fmt.Errorf("unknown mapped relation %q", mapped)
	}
	if rel.From == oldID && !conversionAllowsType(relationType.From, d.To) {
		return false, fmt.Errorf("relation %q cannot originate at %q", mapped, d.To)
	}
	if rel.To == oldID && !conversionAllowsType(relationType.To, d.To) {
		return false, fmt.Errorf("relation %q cannot target %q", mapped, d.To)
	}
	mappedKey := entity.RelationKey{From: from, FromFace: fromFace, Type: mapped, To: to}
	data := &store.RelationData{Properties: rel.Properties, Content: rel.Content}
	if _, err := view.CreateRelation(ctx, mappedKey, data); err != nil {
		return false, fmt.Errorf("convert relation %q: %w", rel.Type, err)
	}
	return false, nil
}

func chooseConvertedFace(rows []*entity.Entity, faceMap map[entity.Face]entity.Face, sourceFace string) *entity.Entity {
	if sourceFace != "" {
		if targetFace, ok := faceMap[entity.Face(sourceFace)]; ok {
			for _, row := range rows {
				if row.Face == targetFace {
					return row
				}
			}
		}
	}
	if len(rows) > 0 {
		return rows[0]
	}
	return nil
}
