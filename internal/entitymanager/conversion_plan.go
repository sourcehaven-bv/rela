package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// errConversionHiddenFace refuses a conversion of an entity with a face the
// caller cannot read. It names no face: which hidden faces exist is data.
var errConversionHiddenFace = fmt.Errorf(
	"%w: converting needs read access to every face of the entity", ErrConversionInvalid)

// hiddenRelationProblem is the one entry a relation the caller cannot see
// adds to a refusal. It says that something blocks the conversion, the one
// bit docs/acl-security.md accepts, and nothing about the relation.
const hiddenRelationProblem = "a relation you cannot see cannot be converted"

// conversionPlan is one conversion resolved against the rows of one read:
// the family, its face mapping, and what happens to each incident relation.
type conversionPlan struct {
	def    metamodel.ConversionDef
	family []*entity.Entity

	faceMap                     map[entity.Face]entity.Face
	requiredFaces, invalidFaces []string

	// moves are the relations the conversion removes: dropped, or replaced
	// by an edge of another type or tail. kept are the relations the family
	// rename carries over unchanged.
	moves   []relationMove
	kept    []*entity.Relation
	invalid []relationProblem
}

// relationMove is one relation a conversion removes, and its replacement
// unless it is dropped. key uses the pre-conversion ids.
type relationMove struct {
	rel     *entity.Relation
	drop    bool
	key     entity.RelationKey
	visible bool
}

// relationProblem is a relation the conversion cannot carry. Text names the
// relation type and is shown only when the caller can see the relation.
type relationProblem struct {
	text    string
	visible bool
}

// plan reads the source family through st and resolves the conversion
// against it. It authorizes the conversion and enforces the read rules of
// [Converter]; a probe ctx (withAffordanceProbe) keeps a denial out of the
// audit log.
func (c *Converter) plan(ctx context.Context, st store.Store, req ConversionRequest) (*conversionPlan, error) {
	m := c.m
	d, ok := m.deps.Meta.Conversions[req.Definition]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownConversion, req.Definition)
	}
	family, err := familyRows(ctx, st, req.SourceID)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, newEntityNotFound(req.SourceID)
	}
	gate := faceGateOf(m)
	if checkErr := requireReadableRow(ctx, gate, req.SourceID, family); checkErr != nil {
		return nil, checkErr
	}
	for _, row := range family {
		if row.Type != d.From {
			return nil, ErrConversionSourceMismatch
		}
	}
	request := acl.WriteRequest{Op: acl.OpConvert, Subject: acl.NewConversionSubject(d.From, req.SourceID, d.To)}
	if checkErr := m.authorizeAndAudit(ctx, request); checkErr != nil {
		return nil, checkErr
	}
	readable, err := readableRows(ctx, gate, family)
	if err != nil {
		return nil, err
	}
	if len(readable) != len(family) {
		return nil, errConversionHiddenFace
	}
	if checkErr := validateConversionOverrides(d, req); checkErr != nil {
		return nil, checkErr
	}
	supplies := req.SourceFace != "" || len(req.Properties) > 0 || req.Content != nil
	if supplies && !familyHasFace(family, req.SourceFace) {
		return nil, fmt.Errorf("%w: %q is not a face of %s", ErrConversionInvalid, req.SourceFace, req.SourceID)
	}
	p := &conversionPlan{def: d, family: family}
	p.faceMap, p.requiredFaces, p.invalidFaces = resolveConversionFaces(d, family, req.FaceMapping)
	rels, err := conversionRelations(ctx, st, req.SourceID)
	if err != nil {
		return nil, err
	}
	vis := newEdgeVisibility(gate, st)
	visErr := vis.seed(ctx, family)
	if visErr == nil {
		visErr = vis.load(ctx, rels)
	}
	if visErr != nil {
		// The error text can name a neighbor the caller cannot see.
		slog.Error("entitymanager: judge relation visibility for a conversion", "id", req.SourceID, "error", visErr)
		return nil, fmt.Errorf("convert %s: %w", req.SourceID, errRelationCheck)
	}
	p.planRelations(m.deps.Meta, req.SourceID, rels, vis)
	return p, nil
}

// requireReadableInputs refuses a conversion that would copy a value the
// caller cannot read: a source field the mapping reads, or the content.
// It compares each stored row with the row as the caller sees it
// ([Deps.CopyVisibility], the caller's redacting reader). Without that
// reader, or on an elevated or cascade handle, every value counts as
// readable, as for every other read on such a handle.
//
// The reader reads the outer store, so this runs before the Tx, never in it.
func (c *Converter) requireReadableInputs(ctx context.Context, p *conversionPlan) error {
	m, d, family := c.m, p.def, p.family
	reader := m.deps.CopyVisibility
	if reader == nil || m.bypassACL || m.cascadeWrite {
		return nil
	}
	fields, usesContent := conversionInputs(d)
	for _, raw := range family {
		seen, ok, err := reader.Get(ctx, raw.Type, raw.ID, raw.Face)
		if err != nil {
			return err
		}
		if !ok {
			return errConversionHiddenFace
		}
		for _, field := range fields {
			value, stored := raw.Properties[field]
			if !stored || metamodel.IsEmptyValue(value) {
				continue
			}
			if !reflect.DeepEqual(value, seen.Properties[field]) {
				return fmt.Errorf("%w: you cannot read source field %q, which the conversion copies",
					ErrConversionInvalid, field)
			}
		}
		if usesContent && raw.Content != seen.Content {
			return fmt.Errorf("%w: you cannot read the content the conversion copies", ErrConversionInvalid)
		}
	}
	return nil
}

// requireInputsUnchanged refuses the transaction when the family it read
// differs from the one requireReadableInputs checked, in its faces or in a
// value the conversion copies. Field visibility can depend on values, so a
// change in between could make the Tx copy a value nobody checked.
func (r *conversionRun) requireInputsUnchanged(p *conversionPlan) error {
	if r.checked == nil {
		return nil
	}
	if len(p.family) != len(r.checked.family) {
		return ErrConversionSourceChanged
	}
	checked := make(map[entity.Face]*entity.Entity, len(r.checked.family))
	for _, row := range r.checked.family {
		checked[row.Face] = row
	}
	fields, usesContent := conversionInputs(p.def)
	for _, row := range p.family {
		before, ok := checked[row.Face]
		if !ok || before.Type != row.Type {
			return ErrConversionSourceChanged
		}
		for _, field := range fields {
			if !reflect.DeepEqual(before.Properties[field], row.Properties[field]) {
				return ErrConversionSourceChanged
			}
		}
		if usesContent && before.Content != row.Content {
			return ErrConversionSourceChanged
		}
	}
	return nil
}

var conversionInputRef = regexp.MustCompile(`\{\{\s*(?:source|new)\.([A-Za-z0-9_-]+)\s*\}\}`)

// conversionInputs returns the source fields d's templates read, and whether
// the target content is taken from the source content.
func conversionInputs(d metamodel.ConversionDef) (fields []string, usesContent bool) {
	templates := slices.Collect(maps.Values(d.Fields))
	usesContent = d.Content == ""
	if !usesContent {
		templates = append(templates, d.Content)
	}
	for _, tmpl := range templates {
		for _, match := range conversionInputRef.FindAllStringSubmatch(tmpl, -1) {
			if match[1] == "content" {
				usesContent = true
				continue
			}
			fields = append(fields, match[1])
		}
	}
	sort.Strings(fields)
	return slices.Compact(fields), usesContent
}

// planRelations sorts the incident relations of id into kept, moves and
// invalid. An identity edge (no tail face) of a faced source travels with
// the family, so it needs no face mapping.
func (p *conversionPlan) planRelations(
	meta *metamodel.Metamodel, id string, rels []*entity.Relation, vis *edgeVisibility,
) {
	d := p.def
	hasImplicitRow := familyHasFace(p.family, "")
	for _, rel := range rels {
		visible := vis.visible(rel)
		mapped, ok := d.Relations[rel.Type]
		if !ok {
			p.invalid = append(p.invalid, relationProblem{rel.Type + " (no mapping)", visible})
			continue
		}
		if mapped == metamodel.ConversionDrop {
			p.moves = append(p.moves, relationMove{rel: rel, drop: true, visible: visible})
			continue
		}
		rd, exists := meta.GetRelationDef(mapped)
		if !exists || rel.From == id && !conversionAllowsType(rd.From, d.To) ||
			rel.To == id && !conversionAllowsType(rd.To, d.To) {

			p.invalid = append(p.invalid, relationProblem{rel.Type + " → " + mapped, visible})
			continue
		}
		face := rel.FromFace
		if rel.From == id && (!rel.FromFace.IsImplicit() || hasImplicitRow) {
			target, found := p.faceMap[rel.FromFace]
			if !found {
				if !familyHasFace(p.family, rel.FromFace.String()) {
					p.invalid = append(p.invalid, relationProblem{rel.Type + " (no face mapping)", visible})
				}
				// Otherwise the face itself is unresolved, which
				// requiredFaces or invalidFaces already reports.
				continue
			}
			face = target
		}
		if mapped == rel.Type && face == rel.FromFace {
			p.kept = append(p.kept, rel)
			continue
		}
		key := entity.RelationKey{From: rel.From, FromFace: face, Type: mapped, To: rel.To}
		p.moves = append(p.moves, relationMove{rel: rel, key: key, visible: visible})
	}
}

// capacityProblems checks every replacement edge against its relation's
// bounds, as if the conversion's removals and earlier replacements were
// already written. The problem text never names the full endpoint: that
// can be an entity the caller cannot see.
func (p *conversionPlan) capacityProblems(
	ctx context.Context, st store.Store, meta *metamodel.Metamodel,
) ([]relationProblem, error) {
	leaving := make(map[entity.RelationKey]bool, len(p.moves))
	for _, mv := range p.moves {
		leaving[mv.rel.Identity()] = true
	}
	var problems []relationProblem
	var pending []entity.RelationKey
	for _, mv := range p.moves {
		if mv.drop {
			continue
		}
		err := checkRelationCapacity(ctx, st, meta, mv.key, leaving, pending)
		var full *CardinalityError
		switch {
		case errors.As(err, &full):
			text := fmt.Sprintf("%s → %s (%s allows at most %d)", mv.rel.Type, mv.key.Type, full.Constraint, full.Limit)
			problems = append(problems, relationProblem{text, mv.visible})
		case err != nil:
			slog.Error("entitymanager: relation capacity check for a conversion", "error", err)
			return nil, fmt.Errorf("convert: %w", errRelationCheck)
		}
		pending = append(pending, mv.key)
	}
	return problems, nil
}

// relationProblemTexts returns the visible problems' texts, plus one
// generic entry when any hidden relation has a problem.
func relationProblemTexts(problems []relationProblem) []string {
	var out []string
	hidden := false
	for _, pr := range problems {
		if pr.visible {
			out = append(out, pr.text)
		} else {
			hidden = true
		}
	}
	sort.Strings(out)
	if hidden {
		out = append(out, hiddenRelationProblem)
	}
	return out
}

// requireCommittable refuses a plan that cannot be written as requested:
// unresolved faces, relations the mapping cannot carry, or a drop of a
// visible relation the caller did not confirm. A dropped relation the
// caller cannot see needs no confirmation, since asking would reveal it.
func (p *conversionPlan) requireCommittable(req ConversionRequest) error {
	if len(p.requiredFaces) > 0 || len(p.invalidFaces) > 0 {
		return fmt.Errorf("%w: face choices required %v; invalid %v",
			ErrConversionInvalid, p.requiredFaces, p.invalidFaces)
	}
	if texts := relationProblemTexts(p.invalid); len(texts) > 0 {
		return fmt.Errorf("%w: %v", ErrConversionInvalid, texts)
	}
	for _, relType := range req.DropRelations {
		if p.def.Relations[relType] != metamodel.ConversionDrop {
			return fmt.Errorf("%w: relation %q is not configured to drop", ErrConversionInvalid, relType)
		}
	}
	for _, mv := range p.moves {
		if mv.drop && mv.visible && !slices.Contains(req.DropRelations, mv.rel.Type) {
			return fmt.Errorf("%w: confirm dropping relation %q", ErrConversionInvalid, mv.rel.Type)
		}
	}
	return nil
}

// removedKeys returns the identities of the relations the conversion removes.
func (p *conversionPlan) removedKeys() []entity.RelationKey {
	keys := make([]entity.RelationKey, 0, len(p.moves))
	for _, mv := range p.moves {
		keys = append(keys, mv.rel.Identity())
	}
	return keys
}

// targetFaceOptions returns the declared target faces of sourceFace, or of
// the first row when the caller named none.
func (p *conversionPlan) targetFaceOptions(sourceFace string) []string {
	if sourceFace == "" && len(p.family) > 0 {
		sourceFace = p.family[0].Face.String()
	}
	return append([]string(nil), p.def.Faces[sourceFace]...)
}

// faceMoves lists the resolved face mapping, sorted by source face.
func (p *conversionPlan) faceMoves() []ConversionMove {
	moves := make([]ConversionMove, 0, len(p.faceMap))
	for src, dst := range p.faceMap {
		moves = append(moves, ConversionMove{SourceFace: src.String(), TargetFace: dst.String()})
	}
	sort.Slice(moves, func(i, j int) bool { return moves[i].SourceFace < moves[j].SourceFace })
	return moves
}

// conversionPropertyValues returns the target properties of one source row:
// the mapped fields, plus the caller's values on the row of sourceFace.
func conversionPropertyValues(
	d metamodel.ConversionDef,
	source *entity.Entity,
	supplied map[string]any,
	sourceFace string,
) map[string]any {
	props := map[string]any{}
	for target, tmpl := range d.Fields {
		if value, ok := copyFieldValue(normalizeConversionSourceRefs(tmpl), source); ok {
			props[target] = value
		}
	}
	if source.Face.String() == sourceFace {
		maps.Copy(props, supplied)
	}
	return props
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

// conversionRelations lists the incident relations of id, each once.
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
