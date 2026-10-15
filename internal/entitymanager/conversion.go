package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var (
	ErrUnknownConversion        = errors.New("entitymanager: no such conversion")
	ErrConversionSourceMismatch = errors.New("entitymanager: conversion source type mismatch")
	ErrConversionInvalid        = errors.New("entitymanager: conversion mapping cannot be applied")
	ErrConversionFaceFamily     = errors.New("entitymanager: conversion face mapping cannot be applied")
)

// ConversionOffer is a schema-declared conversion capability and its ACL verdict.
type ConversionOffer struct {
	Name, Label, To string
	Allowed         bool
	Reason          string
	MappedFields    []string
	ContentMapped   bool
	FaceChoices     map[string][]string
}

// ConversionDefinition returns the effective operator definition held by m.
func ConversionDefinition(m *Manager, name string) (metamodel.ConversionDef, bool) {
	d, ok := m.deps.Meta.Conversions[name]
	return d, ok
}

// ConversionRequest invokes a named conversion. The mapping always comes from
// the metamodel; the caller may provide destination values for form fields.
type ConversionRequest struct {
	Definition, SourceID, TargetID, SourceFace string
	Properties                                 map[string]any
	Content                                    *string
	DropRelations                              []string
	FaceMapping                                map[string]string
}

type ConversionResult struct {
	Definition       string         `json:"definition"`
	OldID            string         `json:"old_id"`
	Entity           *entity.Entity `json:"entity"`
	RelationsDropped int            `json:"relations_dropped"`
}

// ConversionPreview is advisory data shown before the user commits a conversion.
type ConversionPreview struct {
	TargetType       string   `json:"target_type"`
	Required         []string `json:"required"`
	RequiredFaces    []string `json:"required_faces,omitempty"`
	RelationsToDrop  []string `json:"relations_to_drop"`
	InvalidRelations []string `json:"invalid_relations"`
	InvalidFaces     []string `json:"invalid_faces,omitempty"`
	TargetFaces      []string `json:"target_faces"`
}

type ConversionFaceChoice struct {
	SourceFace  string   `json:"source_face"`
	TargetFaces []string `json:"target_faces"`
}

// PreviewConversion checks the named mapping against the current entity and
// reports required form values and relationship consequences without writing.
func PreviewConversion(ctx context.Context, m *Manager, req ConversionRequest) (*ConversionPreview, error) {
	d, ok := m.deps.Meta.Conversions[req.Definition]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownConversion, req.Definition)
	}
	family, err := familyRows(ctx, m.deps.Store, req.SourceID)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, newEntityNotFound(req.SourceID)
	}
	if err := requireReadableRow(ctx, faceGateOf(m), req.SourceID, family); err != nil {
		return nil, err
	}
	for _, row := range family {
		if row.Type != d.From {
			return nil, ErrConversionSourceMismatch
		}
	}
	if err := m.authorizeAndAudit(withAffordanceProbe(ctx), acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewFamilySubject(d.From, req.SourceID)}); err != nil {
		return nil, err
	}
	preview := &ConversionPreview{TargetType: d.To}
	if err := validateConversionOverrides(d, req); err != nil {
		return nil, err
	}
	faceMap, requiredFaces, invalidFaces := resolveConversionFaces(d, family, req.FaceMapping)
	preview.RequiredFaces = requiredFaces
	preview.InvalidFaces = append(preview.InvalidFaces, invalidFaces...)
	if req.SourceFace != "" {
		found := false
		for _, row := range family {
			if row.Face.String() == req.SourceFace {
				found = true
				break
			}
		}
		if !found {
			preview.InvalidFaces = append(preview.InvalidFaces, req.SourceFace+" (unknown current face)")
		}
	}
	if len(family) > 0 {
		if options, ok := d.Faces[family[0].Face.String()]; ok {
			preview.TargetFaces = append([]string(nil), options...)
		}
	}
	toDef, _ := m.deps.Meta.GetEntityDef(d.To)
	for key := range req.Properties {
		if _, known := toDef.Properties[key]; !known {
			return nil, fmt.Errorf("%w: unknown target property %q", ErrConversionInvalid, key)
		}
	}
	for key, p := range toDef.Properties {
		if p.Required && p.Computed == "" {
			for _, source := range family {
				values := conversionPropertyValues(d, source, req.Properties)
				if value, ok := values[key]; !ok || metamodel.IsEmptyValue(value) {
					preview.Required = append(preview.Required, key)
					break
				}
			}
		}
	}
	preview.Required = slices.Compact(slices.Sorted(slices.Values(preview.Required)))
	rels, err := conversionRelations(ctx, m.deps.Store, req.SourceID)
	if err != nil {
		return nil, err
	}
	for _, rel := range rels {
		mapped, ok := d.Relations[rel.Type]
		if !ok {
			preview.InvalidRelations = append(preview.InvalidRelations, rel.Type+" (no mapping)")
			continue
		}
		if mapped == "drop" {
			preview.RelationsToDrop = append(preview.RelationsToDrop, rel.Type)
			continue
		}
		rd, ok := m.deps.Meta.GetRelationDef(mapped)
		if !ok || (rel.From == req.SourceID && !conversionAllowsType(rd.From, d.To)) || (rel.To == req.SourceID && !conversionAllowsType(rd.To, d.To)) {
			preview.InvalidRelations = append(preview.InvalidRelations, rel.Type+" → "+mapped)
		}
	}
	if len(preview.RequiredFaces) == 0 && len(preview.InvalidFaces) == 0 {
		leaving, pending, invalid := conversionRelationPlan(req.SourceID, req.SourceID, rels, d.Relations, faceMap, m.deps.Meta, d.To)
		preview.InvalidRelations = append(preview.InvalidRelations, invalid...)
		for i, key := range pending {
			if err := checkRelationCapacity(ctx, m.deps.Store, m.deps.Meta, key, leaving, pending[:i]); err != nil {
				preview.InvalidRelations = append(preview.InvalidRelations, fmt.Sprintf("%s (%v)", key.Type, err))
			}
		}
	}
	sort.Strings(preview.RelationsToDrop)
	sort.Strings(preview.InvalidRelations)
	sort.Strings(preview.InvalidFaces)
	return preview, nil
}

// ConversionsForSource returns the configured conversions for a source type.
// Allowed is advisory; ConvertEntity rechecks through the same ACL kernel.
func ConversionsForSource(ctx context.Context, m *Manager, sourceType, sourceID string) []ConversionOffer {
	names := make([]string, 0, len(m.deps.Meta.Conversions))
	for name := range m.deps.Meta.Conversions {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]ConversionOffer, 0)
	for _, name := range names {
		d := m.deps.Meta.Conversions[name]
		if d.From != sourceType {
			continue
		}
		o := ConversionOffer{Name: name, Label: d.Label, To: d.To}
		for field := range d.Fields {
			o.MappedFields = append(o.MappedFields, field)
		}
		sort.Strings(o.MappedFields)
		o.ContentMapped = d.Content != ""
		o.FaceChoices = make(map[string][]string, len(d.Faces))
		for sourceFace, choices := range d.Faces {
			o.FaceChoices[sourceFace] = append([]string(nil), choices...)
			sort.Strings(o.FaceChoices[sourceFace])
		}
		if o.Label == "" {
			o.Label = name
		}
		if err := m.authorizeAndAudit(withAffordanceProbe(ctx), acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewFamilySubject(sourceType, sourceID)}); err != nil {
			o.Reason = err.Error()
		} else {
			o.Allowed = true
		}
		out = append(out, o)
	}
	return out
}

// ConvertEntity rekeys a family and retypes its default row in one store Tx.
// Rename lineage hooks preserve old links, comments and version history.
func ConvertEntity(ctx context.Context, m *Manager, req ConversionRequest) (*ConversionResult, error) {
	ctx = withStoreAttribution(ctx)
	d, ok := m.deps.Meta.Conversions[req.Definition]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownConversion, req.Definition)
	}
	if err := validateConversionOverrides(d, req); err != nil {
		return nil, err
	}
	family, err := familyRows(ctx, m.deps.Store, req.SourceID)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, newEntityNotFound(req.SourceID)
	}
	if rerr := requireReadableRow(ctx, faceGateOf(m), req.SourceID, family); rerr != nil {
		return nil, rerr
	}
	for _, row := range family {
		if row.Type != d.From {
			return nil, fmt.Errorf("%w: expected %q, got %q", ErrConversionSourceMismatch, d.From, row.Type)
		}
	}
	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewFamilySubject(d.From, req.SourceID)}); err != nil {
		return nil, err
	}
	toDef, _ := m.deps.Meta.GetEntityDef(d.To)
	faceMap, requiredFaces, invalidFaces := resolveConversionFaces(d, family, req.FaceMapping)
	if len(requiredFaces) > 0 || len(invalidFaces) > 0 {
		return nil, fmt.Errorf("%w: face choices required %v; invalid %v", ErrConversionInvalid, requiredFaces, invalidFaces)
	}
	newID := req.TargetID
	if toDef.IsManualID() {
		if newID == "" {
			return nil, fmt.Errorf("%w: target id is required for manual-id type", ErrConversionInvalid)
		}
		if err := entity.ValidateID(newID); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrConversionInvalid, err)
		}
	} else {
		if newID != "" {
			return nil, fmt.Errorf("%w: target id is generated for this type", ErrConversionInvalid)
		}
		newID, err = generateID(ctx, m.deps, d.To, "", 1)
		if err != nil {
			return nil, err
		}
	}
	if req.SourceID == newID {
		return nil, fmt.Errorf("%w: target id must differ from source id", ErrConversionInvalid)
	}
	taken, err := idTakenByOther(ctx, m.deps.Store, newID, req.SourceID)
	if err != nil {
		return nil, err
	} else if taken {
		return nil, fmt.Errorf("%w: %s", ErrEntityAlreadyExists, newID)
	}
	targets := make([]*entity.Entity, 0, len(family))
	targetByFace := make(map[entity.Face]*entity.Entity, len(family))
	for _, source := range family {
		face, ok := faceMap[source.Face]
		if !ok {
			return nil, fmt.Errorf("%w: no destination face for %q", ErrConversionInvalid, source.Face)
		}
		props := conversionPropertyValues(d, source, req.Properties)
		for key := range req.Properties {
			if _, known := toDef.Properties[key]; !known {
				return nil, fmt.Errorf("%w: unknown target property %q", ErrConversionInvalid, key)
			}
		}
		if err := rejectComputedPresent(m.deps, d.To, props); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrConversionInvalid, err)
		}
		content := source.Content
		if d.Content != "" {
			content = conversionContentValue(d.Content, source)
		}
		if req.Content != nil {
			content = *req.Content
		}
		target := &entity.Entity{ID: newID, Face: face, Type: d.To, Properties: props, Content: content}
		if err := rejectExternalRefChanges(m.deps.Meta, &entity.Entity{Type: d.To, Properties: map[string]any{}}, target, false); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrConversionInvalid, err)
		}
		if err := m.deps.Computed.Evaluate(ctx, target); err != nil {
			return nil, err
		}
		validation := metamodel.NewValidator(m.deps.Meta).ValidateEntity(target.ID, target.Type, target.Properties)
		if hard, _ := partitionValidationErrors(validation); len(hard) > 0 {
			return nil, newValidationError(hard)
		}
		targets = append(targets, target)
		targetByFace[face] = target
	}
	rels, err := conversionRelations(ctx, m.deps.Store, req.SourceID)
	if err != nil {
		return nil, err
	}
	for _, rel := range rels {
		mapped, exists := d.Relations[rel.Type]
		if !exists {
			return nil, fmt.Errorf("%w: relation %q needs an explicit mapping or drop", ErrConversionInvalid, rel.Type)
		}
		if mapped == "drop" {
			if !slices.Contains(req.DropRelations, rel.Type) {
				return nil, fmt.Errorf("%w: confirm dropping relation %q", ErrConversionInvalid, rel.Type)
			}
			continue
		}
		targetRel, exists := m.deps.Meta.GetRelationDef(mapped)
		if !exists {
			return nil, fmt.Errorf("%w: relation %q maps to unknown relation %q", ErrConversionInvalid, rel.Type, mapped)
		}
		if rel.From == req.SourceID && !conversionAllowsType(targetRel.From, d.To) {
			return nil, fmt.Errorf("%w: mapped relation %q cannot originate at %q", ErrConversionInvalid, mapped, d.To)
		}
		if rel.To == req.SourceID && !conversionAllowsType(targetRel.To, d.To) {
			return nil, fmt.Errorf("%w: mapped relation %q cannot target %q", ErrConversionInvalid, mapped, d.To)
		}
	}
	for _, relType := range req.DropRelations {
		if d.Relations[relType] != "drop" {
			return nil, fmt.Errorf("%w: relation %q is not configured to drop", ErrConversionInvalid, relType)
		}
	}
	tx, ok := m.deps.Store.(store.Transactor)
	if !ok {
		return nil, errors.New("entitymanager: store does not support transactions")
	}
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
	dropped := 0
	var converted *entity.Entity
	if err := tx.Tx(ctx, func(view store.Store) error {
		txCtx := store.ContextInTx(ctx)
		current, err := familyRows(txCtx, view, req.SourceID)
		if err != nil {
			return err
		}
		if len(current) == 0 {
			return newEntityNotFound(req.SourceID)
		}
		for _, row := range current {
			if row.Type != d.From {
				return ErrConversionSourceMismatch
			}
		}
		currentFaceMap, missing, invalid := resolveConversionFaces(d, current, req.FaceMapping)
		if len(missing) > 0 || len(invalid) > 0 || !maps.Equal(currentFaceMap, faceMap) {
			return ErrConversionInvalid
		}
		if err := requireReadableRow(txCtx, faceGateOf(m), req.SourceID, current); err != nil {
			return err
		}
		if err := m.authorizeAndAudit(txCtx, acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewFamilySubject(d.From, req.SourceID)}); err != nil {
			return err
		}
		if _, err := view.RenameFamily(txCtx, req.SourceID, newID); err != nil {
			return translateRenameErr(err, req.SourceID, newID)
		}
		replacer, ok := view.(store.FamilyReplacer)
		if !ok {
			return errors.New("entitymanager: store cannot replace entity families")
		}
		replacements := make([]store.FamilyReplacement, 0, len(targets))
		for _, dst := range targets {
			for _, src := range current {
				if faceMap[src.Face] == dst.Face {
					replacements = append(replacements, store.FamilyReplacement{SourceFace: src.Face, Entity: dst})
					break
				}
			}
		}
		if err := replacer.ReplaceEntityFamily(txCtx, newID, replacements); err != nil {
			return err
		}
		convertedRows, err := familyRows(txCtx, view, newID)
		if err != nil {
			return err
		}
		for _, row := range convertedRows {
			if row.Face == faceMap[entity.Face(req.SourceFace)] {
				converted = row
				break
			}
		}
		if converted == nil && len(convertedRows) > 0 {
			converted = convertedRows[0]
		}
		leaving, pending, invalid := conversionRelationPlan(req.SourceID, newID, rels, d.Relations, faceMap, m.deps.Meta, d.To)
		if len(invalid) > 0 {
			return fmt.Errorf("%w: %v", ErrConversionInvalid, invalid)
		}
		for i, key := range pending {
			if err := checkRelationCapacity(txCtx, view, m.deps.Meta, key, leaving, pending[:i]); err != nil {
				return err
			}
		}
		for _, rel := range rels {
			mapped := d.Relations[rel.Type]
			from, to := rel.From, rel.To
			if from == req.SourceID {
				from = newID
			}
			if to == req.SourceID {
				to = newID
			}
			fromFace := rel.FromFace
			if rel.From == req.SourceID {
				fromFace = faceMap[fromFace]
			}
			key := entity.RelationKey{From: from, FromFace: rel.FromFace, Type: rel.Type, To: to}
			// RenameFamily already re-keyed this edge. Keep it in place when
			// the mapping preserves its relation type, retaining its relation ID
			// and history lineage.
			if mapped == rel.Type && fromFace == rel.FromFace {
				continue
			}
			if err := view.DeleteRelation(txCtx, key); err != nil && !errors.Is(err, store.ErrNotFound) {
				return err
			}
			if mapped == "drop" {
				dropped++
				continue
			}
			relType, exists := m.deps.Meta.GetRelationDef(mapped)
			if !exists {
				return fmt.Errorf("unknown mapped relation %q", mapped)
			}
			if rel.From == req.SourceID && !conversionAllowsType(relType.From, d.To) {
				return fmt.Errorf("relation %q cannot originate at %q", mapped, d.To)
			}
			if rel.To == req.SourceID && !conversionAllowsType(relType.To, d.To) {
				return fmt.Errorf("relation %q cannot target %q", mapped, d.To)
			}
			_, err := view.CreateRelation(txCtx, entity.RelationKey{From: from, FromFace: fromFace, Type: mapped, To: to}, &store.RelationData{Properties: rel.Properties, Content: rel.Content})
			if err != nil {
				return fmt.Errorf("convert relation %q: %w", rel.Type, err)
			}
		}
		return nil
	}); err != nil {
		if converted != nil && !familyStillStored(ctx, m.deps.Store, req.SourceID) {
			rn.record(ctx, []*entity.Entity{converted})
		}
		return nil, err
	}
	convertedRows, _ := familyRows(ctx, m.deps.Store, newID)
	rn.record(ctx, convertedRows)
	for src, dst := range faceMap {
		if src != dst {
			notifyAliasesOfFaceMove(ctx, m.deps.AliasRewriter, newID, d.From, src, dst)
		}
	}
	if req.SourceFace != "" {
		if targetFace, ok := faceMap[entity.Face(req.SourceFace)]; ok {
			for _, row := range convertedRows {
				if row.Face == targetFace {
					converted = row
					break
				}
			}
		}
	}
	return &ConversionResult{Definition: req.Definition, OldID: req.SourceID, Entity: converted, RelationsDropped: dropped}, nil
}

func validateConversionOverrides(d metamodel.ConversionDef, req ConversionRequest) error {
	for key := range req.Properties {
		if _, mapped := d.Fields[key]; mapped {
			return fmt.Errorf("%w: target property %q is controlled by the conversion mapping", ErrConversionInvalid, key)
		}
	}
	if d.Content != "" && req.Content != nil {
		return fmt.Errorf("%w: content is controlled by the conversion mapping", ErrConversionInvalid)
	}
	return nil
}

// conversionRelationPlan returns the edge identities a conversion removes
// (drop or type remap) and the replacement identities it will create. Same-
// type edges survive RenameFamily and retain their identity.
func conversionPropertyValues(d metamodel.ConversionDef, source *entity.Entity, supplied map[string]any) map[string]any {
	props := map[string]any{}
	for target, tmpl := range d.Fields {
		if value, ok := copyFieldValue(normalizeConversionSourceRefs(tmpl), source); ok {
			props[target] = value
		}
	}
	for key, value := range supplied {
		props[key] = value
	}
	return props
}

func resolveConversionFaces(d metamodel.ConversionDef, family []*entity.Entity, chosen map[string]string) (map[entity.Face]entity.Face, []string, []string) {
	result := map[entity.Face]entity.Face{}
	var required, invalid []string
	allowedSources := map[string]bool{}
	for _, row := range family {
		source := row.Face.String()
		allowedSources[source] = true
		options, ok := d.Faces[source]
		if !ok || len(options) == 0 {
			invalid = append(invalid, source+" (no declared destination)")
			continue
		}
		pick, provided := chosen[source]
		if !provided && len(options) == 1 {
			pick = options[0]
			provided = true
		}
		if !provided {
			required = append(required, source)
			continue
		}
		if !slices.Contains(options, pick) {
			invalid = append(invalid, source+" → "+pick)
			continue
		}
		target := entity.ImplicitFace
		if pick != "" {
			var err error
			target, err = entity.ParseFace(pick)
			if err != nil {
				invalid = append(invalid, source+" → "+pick)
				continue
			}
		}
		if _, exists := result[target]; exists {
			invalid = append(invalid, pick+" (multiple source faces map here)")
			continue
		}
		result[row.Face] = target
	}
	for source := range chosen {
		if !allowedSources[source] {
			invalid = append(invalid, source+" (unknown source face)")
		}
	}
	sort.Strings(required)
	sort.Strings(invalid)
	return result, required, invalid
}

func conversionRelationPlan(oldID, currentID string, rels []*entity.Relation, mapping map[string]string, faces map[entity.Face]entity.Face, meta *metamodel.Metamodel, targetType string) (map[entity.RelationKey]bool, []entity.RelationKey, []string) {
	leaving := map[entity.RelationKey]bool{}
	var pending []entity.RelationKey
	var invalid []string
	for _, rel := range rels {
		mapped, ok := mapping[rel.Type]
		if !ok {
			invalid = append(invalid, rel.Type+" (no mapping)")
			continue
		}
		if mapped == "drop" {
			continue
		}
		from, to := rel.From, rel.To
		if from == oldID {
			from = currentID
		}
		if to == oldID {
			to = currentID
		}
		face := rel.FromFace
		if rel.From == oldID {
			var found bool
			face, found = faces[rel.FromFace]
			if !found {
				invalid = append(invalid, rel.Type+" (no face mapping)")
				continue
			}
		}
		key := entity.RelationKey{From: from, FromFace: face, Type: mapped, To: to}
		rd, exists := meta.GetRelationDef(mapped)
		if !exists || (rel.From == oldID && !conversionAllowsType(rd.From, targetType)) || (rel.To == oldID && !conversionAllowsType(rd.To, targetType)) {
			invalid = append(invalid, rel.Type+" → "+mapped)
			continue
		}
		leaving[entity.RelationKey{From: from, FromFace: rel.FromFace, Type: rel.Type, To: to}] = true
		if mapped != rel.Type || face != rel.FromFace {
			pending = append(pending, key)
		}
	}
	return leaving, pending, invalid
}

func recordConversionAudit(m *Manager, ctx context.Context, oldID, oldType string, after *entity.Entity) {
	if after == nil {
		return
	}
	summary := "converted from " + oldType
	if !after.Face.IsImplicit() {
		summary += " face " + after.Face.String()
	}
	m.deps.Audit.Record(audit.Record{Time: time.Now().UTC(), Op: audit.OpConvertEntity,
		Before:    &audit.Subject{Kind: "entity", Type: oldType, ID: oldID},
		After:     &audit.Subject{Kind: "entity", Type: after.Type, ID: after.ID},
		Principal: principal.From(ctx), TriggeredBy: audit.TriggeredByFrom(ctx), Summary: summary})
}

func conversionAllowsType(types []string, target string) bool {
	if len(types) == 0 {
		return true
	}
	for _, typ := range types {
		if typ == target {
			return true
		}
	}
	return false
}

func conversionContentValue(template string, source *entity.Entity) string {
	template = normalizeConversionSourceRefs(template)
	if match := singleFieldRef.FindStringSubmatch(template); match != nil && match[1] == "content" {
		return source.Content
	}
	return automation.Interpolate(template, automation.TemplateVars{}, source, nil)
}

var conversionSourceRef = regexp.MustCompile(`\{\{\s*source\.([A-Za-z0-9_-]+)\s*\}\}`)

// normalizeConversionSourceRefs gives conversion mappings an unambiguous
// source.* spelling while retaining compatibility with copy's new.* syntax.
func normalizeConversionSourceRefs(template string) string {
	return conversionSourceRef.ReplaceAllString(template, "{{new.$1}}")
}

func conversionRelations(ctx context.Context, st store.Store, id string) ([]*entity.Relation, error) {
	seen := make(map[entity.RelationKey]bool)
	var out []*entity.Relation
	for _, direction := range []store.Direction{store.DirectionOutgoing, store.DirectionIncoming} {
		for rel, err := range st.ListRelations(ctx, store.RelationQuery{EntityID: id, Direction: direction}) {
			if err != nil {
				return nil, fmt.Errorf("list conversion relations: %w", err)
			}
			key := rel.Identity()
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, rel)
		}
	}
	return out, nil
}
