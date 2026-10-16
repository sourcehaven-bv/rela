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
	d, family, sourceErr := conversionSource(ctx, m, req, true)
	if sourceErr != nil {
		return nil, sourceErr
	}
	if overrideErr := validateConversionOverrides(d, req); overrideErr != nil {
		return nil, overrideErr
	}
	preview := &ConversionPreview{TargetType: d.To}
	faceMap, requiredFaces, invalidFaces := resolveConversionFaces(d, family, req.FaceMapping)
	preview.RequiredFaces = requiredFaces
	preview.InvalidFaces = append(preview.InvalidFaces, invalidFaces...)
	previewConversionFaceChoices(preview, d, family, req.SourceFace)
	toDef, _ := m.deps.Meta.GetEntityDef(d.To)
	preview.Required, sourceErr = conversionRequiredFields(d, family, req.Properties, *toDef)
	if sourceErr != nil {
		return nil, sourceErr
	}
	if relationErr := previewConversionRelations(ctx, m, d, req, faceMap, preview); relationErr != nil {
		return nil, relationErr
	}
	sort.Strings(preview.RelationsToDrop)
	sort.Strings(preview.InvalidRelations)
	sort.Strings(preview.InvalidFaces)
	return preview, nil
}

func conversionSource(
	ctx context.Context,
	m *Manager,
	req ConversionRequest,
	probe bool,
) (metamodel.ConversionDef, []*entity.Entity, error) {
	d, definitionFound := m.deps.Meta.Conversions[req.Definition]
	if !definitionFound {
		return metamodel.ConversionDef{}, nil, fmt.Errorf("%w: %q", ErrUnknownConversion, req.Definition)
	}
	family, familyErr := familyRows(ctx, m.deps.Store, req.SourceID)
	if familyErr != nil {
		return d, nil, familyErr
	}
	if len(family) == 0 {
		return d, nil, newEntityNotFound(req.SourceID)
	}
	if readableErr := requireReadableRow(ctx, faceGateOf(m), req.SourceID, family); readableErr != nil {
		return d, nil, readableErr
	}
	for _, row := range family {
		if row.Type != d.From {
			return d, nil, ErrConversionSourceMismatch
		}
	}
	authorizeCtx := ctx
	if probe {
		authorizeCtx = withAffordanceProbe(ctx)
	}
	request := acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewFamilySubject(d.From, req.SourceID)}
	if authorizeErr := m.authorizeAndAudit(authorizeCtx, request); authorizeErr != nil {
		return d, nil, authorizeErr
	}
	return d, family, nil
}

func previewConversionFaceChoices(
	preview *ConversionPreview,
	d metamodel.ConversionDef,
	family []*entity.Entity,
	sourceFace string,
) {
	if sourceFace != "" && !familyHasFace(family, sourceFace) {
		preview.InvalidFaces = append(preview.InvalidFaces, sourceFace+" (unknown current face)")
	}
	if len(family) > 0 {
		if options, ok := d.Faces[family[0].Face.String()]; ok {
			preview.TargetFaces = append([]string(nil), options...)
		}
	}
}

func familyHasFace(family []*entity.Entity, face string) bool {
	for _, row := range family {
		if row.Face.String() == face {
			return true
		}
	}
	return false
}

func conversionRequiredFields(
	d metamodel.ConversionDef,
	family []*entity.Entity,
	supplied map[string]any,
	target metamodel.EntityDef,
) ([]string, error) {
	for key := range supplied {
		if _, known := target.Properties[key]; !known {
			return nil, fmt.Errorf("%w: unknown target property %q", ErrConversionInvalid, key)
		}
	}
	var required []string
	for key, property := range target.Properties {
		if !property.Required || property.Computed != "" {
			continue
		}
		for _, source := range family {
			values := conversionPropertyValues(d, source, supplied)
			value, exists := values[key]
			if !exists || metamodel.IsEmptyValue(value) {
				required = append(required, key)
				break
			}
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(required))), nil
}

func previewConversionRelations(
	ctx context.Context,
	m *Manager,
	d metamodel.ConversionDef,
	req ConversionRequest,
	faceMap map[entity.Face]entity.Face,
	preview *ConversionPreview,
) error {
	rels, err := conversionRelations(ctx, m.deps.Store, req.SourceID)
	if err != nil {
		return err
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
		if !conversionRelationTypeAllowed(m, d.To, req.SourceID, rel, mapped) {
			preview.InvalidRelations = append(preview.InvalidRelations, rel.Type+" → "+mapped)
		}
	}
	if len(preview.RequiredFaces) > 0 || len(preview.InvalidFaces) > 0 {
		return nil
	}
	leaving, pending, invalid := conversionRelationPlan(
		req.SourceID, req.SourceID, rels, d.Relations, faceMap, m.deps.Meta, d.To,
	)
	preview.InvalidRelations = append(preview.InvalidRelations, invalid...)
	for i, key := range pending {
		if err := checkRelationCapacity(ctx, m.deps.Store, m.deps.Meta, key, leaving, pending[:i]); err != nil {
			preview.InvalidRelations = append(preview.InvalidRelations, fmt.Sprintf("%s (%v)", key.Type, err))
		}
	}
	return nil
}

func conversionRelationTypeAllowed(m *Manager, targetType, sourceID string, rel *entity.Relation, mapped string) bool {
	relationDef, ok := m.deps.Meta.GetRelationDef(mapped)
	if !ok {
		return false
	}
	return (rel.From != sourceID || conversionAllowsType(relationDef.From, targetType)) &&
		(rel.To != sourceID || conversionAllowsType(relationDef.To, targetType))
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
		request := acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewFamilySubject(sourceType, sourceID)}
		if authorizeErr := m.authorizeAndAudit(withAffordanceProbe(ctx), request); authorizeErr != nil {
			o.Reason = authorizeErr.Error()
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
	d, family, sourceErr := conversionSource(ctx, m, req, false)
	if sourceErr != nil {
		return nil, sourceErr
	}
	if overrideErr := validateConversionOverrides(d, req); overrideErr != nil {
		return nil, overrideErr
	}
	toDef, _ := m.deps.Meta.GetEntityDef(d.To)
	faceMap, required, invalid := resolveConversionFaces(d, family, req.FaceMapping)
	if len(required) > 0 || len(invalid) > 0 {
		return nil, fmt.Errorf("%w: face choices required %v; invalid %v", ErrConversionInvalid, required, invalid)
	}
	newID, err := conversionTargetID(ctx, m, d, *toDef, req)
	if err != nil {
		return nil, err
	}
	targets, err := buildConversionTargets(ctx, m, d, req, family, faceMap, newID)
	if err != nil {
		return nil, err
	}
	rels, err := conversionRelations(ctx, m.deps.Store, req.SourceID)
	if err != nil {
		return nil, err
	}
	if err := validateConversionRelationMappings(m, d, req, rels); err != nil {
		return nil, err
	}
	tx, ok := m.deps.Store.(store.Transactor)
	if !ok {
		return nil, errors.New("entitymanager: store does not support transactions")
	}
	rn := prepareConversionRename(m, req, d, newID, faceMap, rels)
	converted, dropped, txErr := convertEntityTransaction(ctx, m, tx, d, req, newID, faceMap, targets, rels)
	if txErr != nil {
		if converted != nil && !familyStillStored(ctx, m.deps.Store, req.SourceID) {
			rn.record(ctx, []*entity.Entity{converted})
		}
		return nil, txErr
	}
	convertedRows, _ := familyRows(ctx, m.deps.Store, newID)
	rn.record(ctx, convertedRows)
	for src, dst := range faceMap {
		if src != dst {
			notifyAliasesOfFaceMove(ctx, m.deps.AliasRewriter, newID, d.From, src, dst)
		}
	}
	converted = chooseConvertedFace(convertedRows, faceMap, req.SourceFace)
	return &ConversionResult{
		Definition:       req.Definition,
		OldID:            req.SourceID,
		Entity:           converted,
		RelationsDropped: dropped,
	}, nil
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
func conversionPropertyValues(
	d metamodel.ConversionDef,
	source *entity.Entity,
	supplied map[string]any,
) map[string]any {
	props := map[string]any{}
	for target, tmpl := range d.Fields {
		if value, ok := copyFieldValue(normalizeConversionSourceRefs(tmpl), source); ok {
			props[target] = value
		}
	}
	maps.Copy(props, supplied)
	return props
}

func resolveConversionFaces(
	d metamodel.ConversionDef,
	family []*entity.Entity,
	chosen map[string]string,
) (result map[entity.Face]entity.Face, required, invalid []string) {
	result = map[entity.Face]entity.Face{}
	allowedSources := map[string]bool{}
	for _, row := range family {
		source := row.Face.String()
		allowedSources[source] = true
		options, ok := d.Faces[source]
		if !ok && len(d.Faces) == 0 && row.Face.IsImplicit() {
			if target, provided := chosen[source]; provided && target != "" {
				invalid = append(invalid, source+" → "+target)
				continue
			}
			result[row.Face] = entity.ImplicitFace
			continue
		}
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

func conversionRelationPlan(
	oldID, currentID string,
	rels []*entity.Relation,
	mapping map[string]string,
	faces map[entity.Face]entity.Face,
	meta *metamodel.Metamodel,
	targetType string,
) (leaving map[entity.RelationKey]bool, pending []entity.RelationKey, invalid []string) {
	leaving = map[entity.RelationKey]bool{}
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
		if !exists {
			invalid = append(invalid, rel.Type+" → "+mapped)
			continue
		}
		invalidFrom := rel.From == oldID && !conversionAllowsType(rd.From, targetType)
		invalidTo := rel.To == oldID && !conversionAllowsType(rd.To, targetType)
		if invalidFrom || invalidTo {
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

func recordConversionAudit(ctx context.Context, m *Manager, oldID, oldType string, after *entity.Entity) {
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
	return len(types) == 0 || slices.Contains(types, target)
}

func conversionContentValue(template string, source *entity.Entity) string {
	template = normalizeConversionSourceRefs(template)
	if match := singleFieldRef.FindStringSubmatch(template); len(match) > 1 && match[1] == "content" {
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
