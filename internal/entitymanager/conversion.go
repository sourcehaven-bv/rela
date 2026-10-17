package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var (
	ErrUnknownConversion        = errors.New("entitymanager: no such conversion")
	ErrConversionSourceMismatch = errors.New("entitymanager: conversion source type mismatch")
	ErrConversionInvalid        = errors.New("entitymanager: conversion mapping cannot be applied")
	ErrConversionFaceFamily     = errors.New("entitymanager: conversion face mapping cannot be applied")
)

// Converter runs the operator-declared type conversions of `conversions:`
// in schema.yaml. A conversion gives an entity a new id and type: the family
// is renamed, each face row is replaced by its mapped target row, and every
// incident relation is kept, remapped or dropped as the definition says.
//
// # Atomicity
//
// The whole conversion runs in one store Tx, and every check runs in it
// before the first write: authorization, read access, the target id, target
// validation, `unique:` values, relation capacity and the authorization of
// the relations it removes. On postgres and sqlite the Tx rolls back on any
// later error. On the file and memory stores a Tx only serializes writers,
// so only an I/O error can still fail after a write, and it leaves a partly
// converted family. Convert then records what reached the store (rename
// lineage, audit, alias rewrite, relation records), as RenameEntity does.
//
// # What the caller must be able to read
//
// A conversion moves content to a new type, and with it to that type's
// readers. So it refuses unless the caller can read every face of the entity
// and every source field and the content the mapping copies: copying a value
// the caller cannot see would hand it to an audience chosen by the caller.
// Incident relations whose far end the caller cannot see are converted as
// declared but never named, counted or listed (see [Converter.Preview]).
//
// It holds the Manager for the reason copyEngine does: authorization and the
// read gate are Manager's, shared with every other write path.
type Converter struct{ m *Manager }

// NewConverter returns the conversion engine that writes through m.
//
// Nil: rejected — a converter without a manager has nothing to write through.
func NewConverter(m *Manager) (*Converter, error) {
	if m == nil {
		return nil, errors.New("entitymanager: NewConverter: manager is required")
	}
	return &Converter{m: m}, nil
}

// ConversionOffer is a schema-declared conversion capability and its ACL verdict.
type ConversionOffer struct {
	Name, Label, To string
	Allowed         bool
	Reason          string
	MappedFields    []string
	ContentMapped   bool
	FaceChoices     map[string][]string
}

// ConversionRequest invokes a named conversion. The mapping always comes from
// the metamodel; the caller may provide destination values for form fields.
// Properties and Content apply to the row of SourceFace only.
type ConversionRequest struct {
	Definition, SourceID, TargetID, SourceFace string
	Properties                                 map[string]any
	Content                                    *string
	DropRelations                              []string
	FaceMapping                                map[string]string
}

// ConversionResult reports a committed conversion.
type ConversionResult struct {
	Definition, OldID string
	// Entity is the converted row of the requested source face, or the
	// first converted row. Nil: never returned with a nil error.
	Entity *entity.Entity
	// RelationsDropped counts the dropped relations the caller can see.
	RelationsDropped int
}

// ConversionPreview is advisory data shown before the user commits a
// conversion. It names only relations the caller can see; a relation the
// caller cannot see that blocks the conversion shows as one generic entry.
type ConversionPreview struct {
	TargetType       string
	Required         []string
	RequiredFaces    []string
	RelationsToDrop  []string
	InvalidRelations []string
	InvalidFaces     []string
	TargetFaces      []string
	// Moves says, per source face, to which target face its comments and
	// version history move. They become visible to whoever may read that
	// face of the target type, which can be a different audience.
	Moves []ConversionMove
}

// ConversionMove is one source face and the target face it becomes.
type ConversionMove struct {
	SourceFace, TargetFace string
}

// Definition returns the conversion definition the converter applies.
func (c *Converter) Definition(name string) (metamodel.ConversionDef, bool) {
	d, ok := c.m.deps.Meta.Conversions[name]
	return d, ok
}

// Offers returns the configured conversions for a source type. Allowed is
// advisory; Convert rechecks through the same ACL kernel.
func (c *Converter) Offers(ctx context.Context, sourceType, sourceID string) []ConversionOffer {
	m := c.m
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
		request := acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewConversionSubject(sourceType, sourceID, d.To)}
		if authorizeErr := m.authorizeAndAudit(withAffordanceProbe(ctx), request); authorizeErr != nil {
			o.Reason = authorizeErr.Error()
		} else {
			o.Allowed = true
		}
		out = append(out, o)
	}
	return out
}

// Preview checks the named mapping against the current entity and reports
// required form values and relationship consequences without writing. Its
// authorization is a probe: a denial is returned but not audited.
func (c *Converter) Preview(ctx context.Context, req ConversionRequest) (*ConversionPreview, error) {
	m := c.m
	p, err := c.plan(withAffordanceProbe(ctx), m.deps.Store, req)
	if err != nil {
		return nil, err
	}
	if err = c.requireReadableInputs(ctx, p); err != nil {
		return nil, err
	}
	d := p.def
	toDef, _ := m.deps.Meta.GetEntityDef(d.To)
	preview := &ConversionPreview{
		TargetType:    d.To,
		RequiredFaces: p.requiredFaces,
		InvalidFaces:  p.invalidFaces,
		TargetFaces:   p.targetFaceOptions(req.SourceFace),
		Moves:         p.faceMoves(),
	}
	preview.Required, err = conversionRequiredFields(d, p.family, req.Properties, req.SourceFace, *toDef)
	if err != nil {
		return nil, err
	}
	for _, mv := range p.moves {
		if mv.drop && mv.visible {
			preview.RelationsToDrop = append(preview.RelationsToDrop, mv.rel.Type)
		}
	}
	problems := p.invalid
	if len(p.requiredFaces) == 0 && len(p.invalidFaces) == 0 {
		capacity, capErr := p.capacityProblems(ctx, m.deps.Store, m.deps.Meta)
		if capErr != nil {
			return nil, capErr
		}
		problems = append(problems, capacity...)
	}
	preview.InvalidRelations = relationProblemTexts(problems)
	sort.Strings(preview.RelationsToDrop)
	return preview, nil
}

// Convert applies the named conversion. See [Converter] for its atomicity
// and read-access rules.
func (c *Converter) Convert(ctx context.Context, req ConversionRequest) (*ConversionResult, error) {
	m := c.m
	ctx = withStoreAttribution(ctx)
	// The plan before the Tx refuses early, and names the relations whose
	// lineage ids must be read while they are still stored. The Tx plans
	// again from what it reads, so nothing below writes from this one.
	pre, err := c.plan(ctx, m.deps.Store, req)
	if err != nil {
		return nil, err
	}
	if err = c.requireReadableInputs(ctx, pre); err != nil {
		return nil, err
	}
	if err = pre.requireCommittable(req); err != nil {
		return nil, err
	}
	lineage, err := lineageIDs(ctx, m.deps.RelationVersionRecorder, pre.removedKeys())
	if err != nil {
		return nil, err
	}
	run := &conversionRun{c: c, req: req}
	txErr := m.deps.Store.Tx(ctx, func(view store.Store) error {
		return run.apply(store.ContextInTx(ctx), view)
	})
	if txErr != nil {
		// A transactional backend rolled everything back. fs and memstore
		// cannot, so a conversion that got as far as the rename is recorded.
		if run.renamed && !familyStillStored(ctx, m.deps.Store, req.SourceID) {
			_, _ = run.record(ctx, lineage)
		}
		return nil, txErr
	}
	rows, err := run.record(ctx, lineage)
	if err != nil {
		return nil, fmt.Errorf("conversion of %s to %s committed, but reading it back failed: %w",
			req.SourceID, run.newID, err)
	}
	converted := chooseConvertedFace(rows, run.plan.faceMap, req.SourceFace)
	if converted == nil {
		return nil, fmt.Errorf("conversion of %s to %s committed, but %w", req.SourceID, run.newID,
			newEntityNotFound(run.newID))
	}
	return &ConversionResult{
		Definition:       req.Definition,
		OldID:            req.SourceID,
		Entity:           converted,
		RelationsDropped: run.visibleDrops,
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

func conversionRequiredFields(
	d metamodel.ConversionDef,
	family []*entity.Entity,
	supplied map[string]any,
	sourceFace string,
	target metamodel.EntityDef,
) ([]string, error) {
	if err := rejectUnknownTargetProperties(supplied, target); err != nil {
		return nil, err
	}
	var required []string
	for key, property := range target.Properties {
		if !property.Required || property.Computed != "" {
			continue
		}
		for _, source := range family {
			values := conversionPropertyValues(d, source, supplied, sourceFace)
			value, exists := values[key]
			if !exists || metamodel.IsEmptyValue(value) {
				required = append(required, key)
				break
			}
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(required))), nil
}

func rejectUnknownTargetProperties(supplied map[string]any, target metamodel.EntityDef) error {
	for key := range supplied {
		if _, known := target.Properties[key]; !known {
			return fmt.Errorf("%w: unknown target property %q", ErrConversionInvalid, key)
		}
	}
	return nil
}

func familyHasFace(family []*entity.Entity, face string) bool {
	for _, row := range family {
		if row.Face.String() == face {
			return true
		}
	}
	return false
}

// resolveConversionFaces maps every row of family to its target face: the
// caller's choice, or the only declared option. Two source faces never land
// on one target face, whatever order the rows come in.
func resolveConversionFaces(
	d metamodel.ConversionDef,
	family []*entity.Entity,
	chosen map[string]string,
) (result map[entity.Face]entity.Face, required, invalid []string) {
	result = map[entity.Face]entity.Face{}
	usedTargets := map[entity.Face]bool{}
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
			usedTargets[entity.ImplicitFace] = true
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
		if usedTargets[target] {
			invalid = append(invalid, pick+" (multiple source faces map here)")
			continue
		}
		usedTargets[target] = true
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

func conversionAllowsType(types []string, target string) bool {
	return len(types) == 0 || slices.Contains(types, target)
}

func chooseConvertedFace(rows []*entity.Entity, faceMap map[entity.Face]entity.Face, sourceFace string) *entity.Entity {
	if targetFace, ok := faceMap[entity.Face(sourceFace)]; ok {
		for _, row := range rows {
			if row.Face == targetFace {
				return row
			}
		}
	}
	if len(rows) > 0 {
		return rows[0]
	}
	return nil
}
