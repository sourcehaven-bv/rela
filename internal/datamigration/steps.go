package datamigration

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Step is one executable unit of a migration file. Every step MUST be
// idempotent by construction — a crashed run's recovery story is "run it
// again", so re-applying a step to already-transformed data is a no-op
// (rename fires only when the old key is present, map_values only maps old
// values, set_default only fills gaps, drops of absent data do nothing).
type Step interface {
	// Kind is the YAML key that declared the step (e.g. "rename_property").
	Kind() string
	// Target is a short human label for reports ("task.status → task.state").
	Target() string
	// Validate checks the step's targets against the migration's embedded
	// projections at parse time — a typo'd type or property is a parse
	// error, never a silent runtime no-op.
	Validate(from, to metamodel.ShapeProjection) error
	// Run executes the step. With x.Apply false it only counts what would
	// change.
	Run(ctx context.Context, x *Exec) (StepResult, error)
}

// StepResult reports one executed step.
type StepResult struct {
	Kind     string
	Target   string
	Affected int      // records that changed (or would change, in dry-run)
	Notes    []string // per-step anomalies: unmapped values, unconvertible values, conflicts
}

// parseStep decodes one `steps:` list element. Each element is a single-key
// mapping (`- rename_property: {...}`); the key selects the step type and
// the body decodes strictly (unknown fields are errors).
func parseStep(node *yaml.Node) (Step, error) {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return nil, errors.New("a step must be a single-key mapping like `rename_property: {...}`")
	}
	kind := node.Content[0].Value
	body := node.Content[1]

	var step Step
	switch kind {
	case "rename_property":
		step = &renamePropertyStep{}
	case "rename_entity_type":
		step = &renameEntityTypeStep{}
	case "rename_face":
		step = &renameFaceStep{}
	case "rename_relation_type":
		step = &renameRelationTypeStep{}
	case "map_values":
		step = &mapValuesStep{}
	case "set_default":
		step = &setDefaultStep{}
	case "recompute_computed":
		step = &recomputeComputedStep{}
	case "convert":
		step = &convertStep{}
	case "drop_property":
		step = &dropPropertyStep{}
	case "drop_entities":
		step = &dropEntitiesStep{}
	case "drop_relations":
		step = &dropRelationsStep{}
	case "lua":
		step = &luaStep{}
	default:
		return nil, fmt.Errorf("unknown step kind %q", kind)
	}
	if err := decodeStrict(body, step); err != nil {
		return nil, fmt.Errorf("%s: %w", kind, err)
	}
	return step, nil
}

// decodeStrict decodes a YAML node into dst rejecting unknown fields.
// yaml.Node.Decode does not honor KnownFields, so the node is re-encoded and
// decoded through a Decoder — the only strictness hook yaml.v3 offers.
func decodeStrict(node *yaml.Node, dst any) error {
	var buf strings.Builder
	enc := yaml.NewEncoder(&buf)
	if err := enc.Encode(node); err != nil {
		return err
	}
	_ = enc.Close()
	dec := yaml.NewDecoder(strings.NewReader(buf.String()))
	dec.KnownFields(true)
	return dec.Decode(dst)
}

// ---- shape lookup helpers (parse-time validation) ----

func entityInShape(p metamodel.ShapeProjection, typ string) bool {
	_, ok := p.Entities[typ]
	return ok
}

func entityPropInShape(p metamodel.ShapeProjection, typ, prop string) bool {
	es, ok := p.Entities[typ]
	if !ok {
		return false
	}
	_, ok = es.Properties[prop]
	return ok
}

// ---- rename_property ----

type renamePropertyStep struct {
	Entity string `yaml:"entity"`
	From   string `yaml:"from"`
	To     string `yaml:"to"`
}

func (s *renamePropertyStep) Kind() string   { return "rename_property" }
func (s *renamePropertyStep) Target() string { return s.Entity + "." + s.From + " → " + s.To }

func (s *renamePropertyStep) Validate(from, to metamodel.ShapeProjection) error {
	if s.Entity == "" || s.From == "" || s.To == "" {
		return errors.New("entity, from and to are required")
	}
	if !entityPropInShape(from, s.Entity, s.From) {
		return fmt.Errorf("property %s.%s is not in the from-schema", s.Entity, s.From)
	}
	if !entityPropInShape(to, s.Entity, s.To) {
		return fmt.Errorf("property %s.%s is not in the to-schema", s.Entity, s.To)
	}
	return nil
}

func (s *renamePropertyStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	err := x.forEachEntity(ctx, s.Entity, func(e *entity.Entity) (bool, error) {
		v, has := e.Properties[s.From]
		if !has {
			return false, nil
		}
		if _, taken := e.Properties[s.To]; taken {
			// Both keys present: a rename in either direction would destroy
			// one of the two values, so touch NOTHING and surface the
			// conflict — the old key stays visible (and eventually GC-able
			// as orphaned drift) until a human decides. Idempotent: every
			// re-run reports the same conflict.
			res.Notes = append(res.Notes, fmt.Sprintf(
				"%s: both %q and %q are set — left untouched, resolve by hand", e.ID, s.From, s.To))
			return false, nil
		}
		e.Properties[s.To] = v
		delete(e.Properties, s.From)
		return true, nil
	}, &res)
	return res, err
}

// ---- rename_entity_type ----

type renameEntityTypeStep struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

func (s *renameEntityTypeStep) Kind() string   { return "rename_entity_type" }
func (s *renameEntityTypeStep) Target() string { return s.From + " → " + s.To }

func (s *renameEntityTypeStep) Validate(from, to metamodel.ShapeProjection) error {
	if s.From == "" || s.To == "" {
		return errors.New("from and to are required")
	}
	if !entityInShape(from, s.From) {
		return fmt.Errorf("entity type %q is not in the from-schema", s.From)
	}
	if !entityInShape(to, s.To) {
		return fmt.Errorf("entity type %q is not in the to-schema", s.To)
	}
	return nil
}

func (s *renameEntityTypeStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	// A type rename keeps every entity ID, so this is an in-place update:
	// the store relocates the record (fsstore moves the file — the
	// type-change-on-update contract pinned in storetest), relations keep
	// their endpoints, and the version sweep captures the change as a
	// normal update on an unbroken lineage.
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	err := x.forEachEntity(ctx, s.From, func(e *entity.Entity) (bool, error) {
		e.Type = s.To
		return true, nil
	}, &res)
	return res, err
}

// ---- rename_face ----

type renameFaceStep struct {
	Entity string `yaml:"entity"`
	From   string `yaml:"from"`
	To     string `yaml:"to"`

	// Stored coordinates, resolved in Validate where both shapes are in hand.
	// Exec carries no projections, and resolving here rather than widening it
	// keeps the bare-face mapping next to the schemas that define it.
	fromStored string
	toStored   string
}

func (s *renameFaceStep) Kind() string { return "rename_face" }
func (s *renameFaceStep) Target() string {
	return s.Entity + "." + s.From + " → " + s.To
}

func (s *renameFaceStep) Validate(from, to metamodel.ShapeProjection) error {
	if s.Entity == "" || s.From == "" || s.To == "" {
		return errors.New("entity, from and to are required")
	}
	if !faceInShape(from, s.Entity, s.From) {
		return fmt.Errorf("entity %q declares no face %q in the from-schema", s.Entity, s.From)
	}
	if !faceInShape(to, s.Entity, s.To) {
		return fmt.Errorf("entity %q declares no face %q in the to-schema", s.Entity, s.To)
	}
	s.fromStored = storedFaceIn(from, s.Entity, s.From)
	s.toStored = storedFaceIn(to, s.Entity, s.To)
	if s.fromStored == "" && s.toStored != "" {
		// The bare face is a family's identity row: every store refuses to
		// delete it while sibling faces remain, so "move the bare rows to a
		// named coordinate" fails on the first entity with a sibling and
		// leaves a duplicate behind. Renaming the bare face away is a change
		// to `bare_face:` plus a family rewrite, not a row move.
		return fmt.Errorf("entity %q: face %q is the bare face; it cannot be renamed to a "+
			"named coordinate by moving rows — change `bare_face:` in the schema instead",
			s.Entity, s.From)
	}
	return nil
}

// Run moves every row at the old face to the new coordinate (TKT-O0A8FO).
//
// # Why this is not forEachEntity
//
// A face is a stored COORDINATE, and store.UpdateEntity addresses a row BY that
// coordinate — handing it an entity whose Face has been changed looks up the
// NEW coordinate, which does not exist yet, and returns not-found (verified
// against memstore). So unlike rename_entity_type, which really is an in-place
// field update, a face rename is a MOVE: create the row at the new coordinate,
// then delete the old one.
//
// Create-then-delete rather than delete-then-create, so a failure between the
// two leaves the data duplicated rather than destroyed. Re-running the
// migration then converges — a destination row holding the source's content
// counts as already moved, and only the source is deleted — which is the
// idempotence the engine requires of every step.
//
// # The bare face is the asymmetric case
//
// The face named by `bare_face:` is stored as the ZERO coordinate, not under
// its own name, so renaming to or from it moves rows between the zero
// coordinate and a named one. Both sides are resolved through the schema that
// declares them (from-side against the from-shape, to-side against the
// to-shape) in Validate. When the two resolve to the SAME coordinate the rename
// is a no-op in storage terms — a face that was already bare being renamed while
// staying bare — and the step does nothing rather than deleting the row it just
// re-created.
func (s *renameFaceStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	if s.fromStored == s.toStored {
		return res, nil
	}

	var moving []*entity.Entity
	q := store.EntityQuery{Type: s.Entity, AllStates: true}
	for e, err := range x.Store.ListEntities(ctx, q) {
		if err != nil {
			return res, err
		}
		if string(e.Face) == s.fromStored {
			moving = append(moving, e)
		}
	}
	res.Affected = len(moving)
	if !x.Apply {
		return res, nil
	}

	for _, e := range moving {
		// A row already at the destination is EITHER the previous run's own
		// copy (a crash between create and delete leaves both rows — the
		// re-run must converge, which is the idempotence the engine requires)
		// OR a genuine COLLISION: the entity has DIFFERENT content at both
		// coordinates and the rename would silently destroy one of them.
		// Identical content is the former; anything else is refused with the
		// ids named, so the operator resolves it deliberately.
		//
		// A collision is most often a rename ONTO the bare face while the bare
		// row still holds the type's original content — the flat/faced
		// boundary, which is exactly where this arc's other defects clustered.
		existing, err := x.Store.GetEntityState(ctx, e.ID, entity.Face(s.toStored))
		alreadyMoved := err == nil && existing != nil && sameContent(existing, e)
		if err == nil && existing != nil && !alreadyMoved {
			return res, fmt.Errorf(
				"%s: cannot rename face %q to %q — a row already exists at the destination; "+
					"drop or merge it first, or this rename would destroy one of the two",
				e.ID, s.From, s.To)
		}
		if !alreadyMoved {
			moved := *e
			moved.Face = entity.Face(s.toStored)
			if err := x.Store.CreateEntity(ctx, &moved); err != nil {
				return res, fmt.Errorf("%s: create at %q: %w", e.ID, s.toStored, err)
			}
		}
		if _, err := x.Store.DeleteEntityState(ctx, e.ID, entity.Face(s.fromStored)); err != nil {
			return res, fmt.Errorf("%s: remove old face %q: %w", e.ID, s.fromStored, err)
		}
	}
	return res, nil
}

// sameContent reports whether two rows carry the same properties and body —
// the test for "this destination row is the previous run's copy".
func sameContent(a, b *entity.Entity) bool {
	return a.Content == b.Content && reflect.DeepEqual(a.Properties, b.Properties)
}

// faceInShape reports whether a projection declares the named face for a type.
func faceInShape(p metamodel.ShapeProjection, typ, face string) bool {
	es, ok := p.Entities[typ]
	if !ok {
		return false
	}
	return slices.Contains(es.Faces, face)
}

// storedFaceIn maps a DECLARED face name to the coordinate it is stored under,
// per the given shape: the type's `bare_face` stores as the empty string, every
// other face under its own name.
//
// The shape-projection twin of metamodel.StoredFace, which needs a whole
// *Metamodel a migration step does not have. It is derivable here because
// ShapeProjection carries BareFace precisely so this question is answerable
// from a migration file alone.
func storedFaceIn(p metamodel.ShapeProjection, typ, declared string) string {
	if es, ok := p.Entities[typ]; ok && es.BareFace == declared {
		return ""
	}
	return declared
}

// ---- rename_relation_type ----

type renameRelationTypeStep struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

func (s *renameRelationTypeStep) Kind() string   { return "rename_relation_type" }
func (s *renameRelationTypeStep) Target() string { return s.From + " → " + s.To }

func (s *renameRelationTypeStep) Validate(from, to metamodel.ShapeProjection) error {
	if s.From == "" || s.To == "" {
		return errors.New("from and to are required")
	}
	if _, ok := from.Relations[s.From]; !ok {
		return fmt.Errorf("relation type %q is not in the from-schema", s.From)
	}
	if _, ok := to.Relations[s.To]; !ok {
		return fmt.Errorf("relation type %q is not in the to-schema", s.To)
	}
	return nil
}

func (s *renameRelationTypeStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	// A relation's type is part of its identity triple, so this is
	// necessarily recreate-then-delete per relation (idempotent: a re-run
	// finds no relations of the old type). On pg the old lifetime's history
	// is closed with a synchronous delete capture; the new triple starts a
	// fresh lineage — a known, documented cost of relation-type renames.
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	rels, err := collectRelations(ctx, x.Store, s.From)
	if err != nil {
		return res, err
	}
	res.Affected = len(rels)
	if !x.Apply {
		return res, nil
	}
	for _, r := range rels {
		data := &store.RelationData{Properties: r.Properties, Content: r.Content}
		if _, err := x.Store.CreateRelation(ctx, r.From, s.To, r.To, data); err != nil {
			if !errors.Is(err, store.ErrConflict) {
				return res, fmt.Errorf("create %s--%s--%s: %w", r.From, s.To, r.To, err)
			}
			// Already created by a prior (crashed) run — fall through to delete.
		}
		if err := x.captureRelationDelete(ctx, r); err != nil {
			return res, err
		}
		if err := x.Store.DeleteRelation(ctx, r.From, s.From, r.To); err != nil && !errors.Is(err, store.ErrNotFound) {
			return res, fmt.Errorf("delete %s--%s--%s: %w", r.From, s.From, r.To, err)
		}
	}
	return res, nil
}

// ---- map_values ----

type mapValuesStep struct {
	Entity   string            `yaml:"entity"`
	Property string            `yaml:"property"`
	Mapping  map[string]string `yaml:"mapping"`
}

func (s *mapValuesStep) Kind() string   { return "map_values" }
func (s *mapValuesStep) Target() string { return s.Entity + "." + s.Property }

func (s *mapValuesStep) Validate(_, to metamodel.ShapeProjection) error {
	if s.Entity == "" || s.Property == "" || len(s.Mapping) == 0 {
		return errors.New("entity, property and a non-empty mapping are required")
	}
	// The property must exist in the to-schema (values are being mapped
	// INTO the new value set); it may be absent from the from-schema when
	// the step follows a rename_property in the same file.
	if !entityPropInShape(to, s.Entity, s.Property) {
		return fmt.Errorf("property %s.%s is not in the to-schema", s.Entity, s.Property)
	}
	return nil
}

func (s *mapValuesStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	err := x.forEachEntity(ctx, s.Entity, func(e *entity.Entity) (bool, error) {
		v, has := e.Properties[s.Property]
		if !has {
			return false, nil
		}
		switch val := v.(type) {
		case string:
			if mapped, ok := s.Mapping[val]; ok {
				e.Properties[s.Property] = mapped
				return true, nil
			}
		case []any:
			changed := false
			out := make([]any, len(val))
			for i, item := range val {
				out[i] = item
				if str, ok := item.(string); ok {
					if mapped, ok := s.Mapping[str]; ok {
						out[i] = mapped
						changed = true
					}
				}
			}
			if changed {
				e.Properties[s.Property] = out
				return true, nil
			}
		}
		return false, nil
	}, &res)
	return res, err
}

// ---- set_default ----

type setDefaultStep struct {
	Entity   string `yaml:"entity"`
	Property string `yaml:"property"`
	Value    string `yaml:"value"`
	// OnlyMissing (default true) fills only entities without a value. An
	// explicit false overwrites every entity — reviewable in the file, but
	// rarely what a backfill wants.
	OnlyMissing *bool `yaml:"only_missing,omitempty"`
}

func (s *setDefaultStep) Kind() string   { return "set_default" }
func (s *setDefaultStep) Target() string { return s.Entity + "." + s.Property }

func (s *setDefaultStep) onlyMissing() bool { return s.OnlyMissing == nil || *s.OnlyMissing }

func (s *setDefaultStep) Validate(_, to metamodel.ShapeProjection) error {
	if s.Entity == "" || s.Property == "" || s.Value == "" {
		return errors.New("entity, property and value are required")
	}
	if !entityPropInShape(to, s.Entity, s.Property) {
		return fmt.Errorf("property %s.%s is not in the to-schema", s.Entity, s.Property)
	}
	return nil
}

func (s *setDefaultStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	err := x.forEachEntity(ctx, s.Entity, func(e *entity.Entity) (bool, error) {
		cur, has := e.Properties[s.Property]
		if s.onlyMissing() && has && !isEmptyValue(cur) {
			return false, nil
		}
		if has && cur == s.Value {
			return false, nil
		}
		if e.Properties == nil {
			e.Properties = map[string]any{}
		}
		e.Properties[s.Property] = s.Value
		return true, nil
	}, &res)
	return res, err
}

// ---- recompute_computed ----

// recomputeComputedStep deliberately targets an entity type, not one
// property. Re-evaluating the complete graph is what keeps both dependencies
// and downstream computed properties coherent after any expression changes.
type recomputeComputedStep struct {
	Entity string `yaml:"entity"`
}

func (s *recomputeComputedStep) Kind() string   { return "recompute_computed" }
func (s *recomputeComputedStep) Target() string { return s.Entity }

func (s *recomputeComputedStep) Validate(_, to metamodel.ShapeProjection) error {
	if s.Entity == "" {
		return errors.New("entity is required")
	}
	shape, ok := to.Entities[s.Entity]
	if !ok {
		return fmt.Errorf("entity type %q is not in the to-schema", s.Entity)
	}
	for _, prop := range shape.Properties {
		if prop.Computed != "" {
			return nil
		}
	}
	return fmt.Errorf("entity type %q has no computed properties in the to-schema", s.Entity)
}

func (s *recomputeComputedStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	if x.Computed == nil {
		return res, errors.New("computed property evaluator is not configured")
	}
	err := x.forEachEntity(ctx, s.Entity, func(e *entity.Entity) (bool, error) {
		before := maps.Clone(e.Properties)
		if err := x.Computed.Evaluate(ctx, e); err != nil {
			return false, err
		}
		return !reflect.DeepEqual(before, e.Properties), nil
	}, &res)
	return res, err
}

// ---- convert ----

type convertStep struct {
	Entity   string `yaml:"entity"`
	Property string `yaml:"property"`
	ToType   string `yaml:"to_type"`
	// FromFormat/ToFormat override date layout detection/output (Go layouts).
	FromFormat string `yaml:"from_format,omitempty"`
	ToFormat   string `yaml:"to_format,omitempty"`

	// toList/fromList are resolved from the projections at Validate time so
	// Run can restructure scalar↔list without re-consulting the schema.
	toList bool
}

func (s *convertStep) Kind() string { return "convert" }
func (s *convertStep) Target() string {
	return fmt.Sprintf("%s.%s → %s", s.Entity, s.Property, s.ToType)
}

func (s *convertStep) Validate(_, to metamodel.ShapeProjection) error {
	if s.Entity == "" || s.Property == "" || s.ToType == "" {
		return errors.New("entity, property and to_type are required")
	}
	es, ok := to.Entities[s.Entity]
	if !ok {
		return fmt.Errorf("entity type %q is not in the to-schema", s.Entity)
	}
	ps, ok := es.Properties[s.Property]
	if !ok {
		return fmt.Errorf("property %s.%s is not in the to-schema", s.Entity, s.Property)
	}
	s.toList = ps.List
	if s.ToFormat == "" {
		s.ToFormat = ps.Format
	}
	switch s.ToType {
	case "string", "integer", "boolean", "date", "datetime":
	default:
		return fmt.Errorf("no built-in coercion to %q (use a lua step)", s.ToType)
	}
	return nil
}

func (s *convertStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	err := x.forEachEntity(ctx, s.Entity, func(e *entity.Entity) (bool, error) {
		v, has := e.Properties[s.Property]
		if !has || v == nil {
			return false, nil
		}
		converted, changed, note := s.convertValue(v)
		if note != "" {
			res.Notes = append(res.Notes, fmt.Sprintf("%s: %s", e.ID, note))
		}
		if !changed {
			return false, nil
		}
		e.Properties[s.Property] = converted
		return true, nil
	}, &res)
	return res, err
}

// convertValue coerces one stored value. Unconvertible values are LEFT IN
// PLACE with a note — a migration never destroys data it cannot transform.
func (s *convertStep) convertValue(v any) (out any, changed bool, note string) {
	// Restructure list-ness first, then coerce each scalar.
	items, wasList := v.([]any)
	if !wasList {
		items = []any{v}
	}
	converted := make([]any, 0, len(items))
	anyChanged := wasList != s.toList
	for _, item := range items {
		c, ok := coerceScalar(item, s.ToType, s.FromFormat, s.ToFormat)
		if !ok {
			return v, false, fmt.Sprintf("cannot convert %v to %s — left unchanged", item, s.ToType)
		}
		if c != item {
			anyChanged = true
		}
		converted = append(converted, c)
	}
	if !anyChanged {
		return v, false, ""
	}
	if s.toList {
		return converted, true, ""
	}
	if len(converted) == 1 {
		return converted[0], true, ""
	}
	return v, false, fmt.Sprintf("value has %d elements but the target is scalar — left unchanged", len(converted))
}

// commonDateLayouts are tried in order when no from_format is declared.
var commonDateLayouts = []string{
	metamodel.DefaultDateFormat, time.RFC3339, "02-01-2006", "01/02/2006", "2006/01/02", "Jan 2, 2006",
}

func coerceScalar(v any, toType, fromFormat, toFormat string) (any, bool) {
	switch toType {
	case "string":
		switch val := v.(type) {
		case string:
			return val, true
		case bool:
			return strconv.FormatBool(val), true
		case int, int64, float64:
			return fmt.Sprintf("%v", val), true
		}
	case "integer":
		return coerceInteger(v)
	case "boolean":
		return coerceBoolean(v)
	case "date", "datetime":
		return coerceDate(v, toType, fromFormat, toFormat)
	}
	return nil, false
}

func coerceInteger(v any) (any, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case float64:
		if val == float64(int(val)) {
			return int(val), true
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			return n, true
		}
	}
	return nil, false
}

func coerceBoolean(v any) (any, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "true", "yes", "1":
			return true, true
		case "false", "no", "0":
			return false, true
		}
	}
	return nil, false
}

func coerceDate(v any, toType, fromFormat, toFormat string) (any, bool) {
	str, ok := v.(string)
	if !ok {
		return nil, false
	}
	str = strings.TrimSpace(str)
	layouts := commonDateLayouts
	if fromFormat != "" {
		layouts = []string{fromFormat}
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, str)
		if err != nil {
			continue
		}
		out := toFormat
		if out == "" {
			if toType == "datetime" {
				out = metamodel.DefaultDatetimeFormat
			} else {
				out = metamodel.DefaultDateFormat
			}
		}
		return t.Format(out), true
	}
	return nil, false
}

// ---- drop_property ----

type dropPropertyStep struct {
	Entity   string `yaml:"entity"`
	Property string `yaml:"property"`
}

func (s *dropPropertyStep) Kind() string   { return "drop_property" }
func (s *dropPropertyStep) Target() string { return s.Entity + "." + s.Property }

func (s *dropPropertyStep) Validate(_, to metamodel.ShapeProjection) error {
	if s.Entity == "" || s.Property == "" {
		return errors.New("entity and property are required")
	}
	if entityPropInShape(to, s.Entity, s.Property) {
		return fmt.Errorf("property %s.%s still exists in the to-schema — dropping it would erase live data", s.Entity, s.Property)
	}
	return nil
}

func (s *dropPropertyStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	err := x.forEachEntity(ctx, s.Entity, func(e *entity.Entity) (bool, error) {
		if _, has := e.Properties[s.Property]; !has {
			return false, nil
		}
		delete(e.Properties, s.Property)
		return true, nil
	}, &res)
	return res, err
}

// ---- drop_entities ----

type dropEntitiesStep struct {
	Type string `yaml:"type"`
}

func (s *dropEntitiesStep) Kind() string   { return "drop_entities" }
func (s *dropEntitiesStep) Target() string { return s.Type }

func (s *dropEntitiesStep) Validate(_, to metamodel.ShapeProjection) error {
	if s.Type == "" {
		return errors.New("type is required")
	}
	if entityInShape(to, s.Type) {
		return fmt.Errorf("entity type %q still exists in the to-schema — only types unknown to the new schema may be dropped", s.Type)
	}
	return nil
}

func (s *dropEntitiesStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	ids, err := collectEntityIDs(ctx, x.Store, s.Type)
	if err != nil {
		return res, err
	}
	res.Affected = len(ids)
	if !x.Apply {
		return res, nil
	}
	for _, id := range ids {
		// Capture BEFORE the delete: the row is gone afterwards and no
		// sweep can reconstruct it (amendment A1).
		e, err := x.Store.GetEntity(ctx, id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue // already deleted by a prior crashed run
			}
			return res, err
		}
		if capErr := x.captureEntityDelete(ctx, e); capErr != nil {
			return res, capErr
		}
		del, err := x.Store.DeleteEntity(ctx, id, true)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			return res, err
		}
		for _, r := range del.DeletedRelations {
			// The relation is already gone (the store cascade-deleted it);
			// capture is best-effort here, logged inside the capturer's
			// error path via the returned error.
			if cerr := x.captureRelationDelete(ctx, r); cerr != nil {
				res.Notes = append(res.Notes, cerr.Error())
			}
		}
	}
	return res, nil
}

// ---- drop_relations ----

type dropRelationsStep struct {
	Type string `yaml:"type"`
}

func (s *dropRelationsStep) Kind() string   { return "drop_relations" }
func (s *dropRelationsStep) Target() string { return s.Type }

func (s *dropRelationsStep) Validate(_, to metamodel.ShapeProjection) error {
	if s.Type == "" {
		return errors.New("type is required")
	}
	if _, ok := to.Relations[s.Type]; ok {
		return fmt.Errorf("relation type %q still exists in the to-schema — only types unknown to the new schema may be dropped", s.Type)
	}
	return nil
}

func (s *dropRelationsStep) Run(ctx context.Context, x *Exec) (StepResult, error) {
	res := StepResult{Kind: s.Kind(), Target: s.Target()}
	rels, err := collectRelations(ctx, x.Store, s.Type)
	if err != nil {
		return res, err
	}
	res.Affected = len(rels)
	if !x.Apply {
		return res, nil
	}
	for _, r := range rels {
		if capErr := x.captureRelationDelete(ctx, r); capErr != nil {
			return res, capErr
		}
		delErr := x.Store.DeleteRelation(ctx, r.From, r.Type, r.To)
		if delErr != nil && !errors.Is(delErr, store.ErrNotFound) {
			return res, delErr
		}
	}
	return res, nil
}

// isEmptyValue reports whether a stored property value counts as "missing"
// for set_default's only_missing semantics.
func isEmptyValue(v any) bool {
	switch val := v.(type) {
	case nil:
		return true
	case string:
		return val == ""
	case []any:
		return len(val) == 0
	}
	return false
}

// ---- shared collection helpers ----

func collectEntityIDs(ctx context.Context, st store.Store, typ string) ([]string, error) {
	var ids []string
	for e, err := range st.ListEntities(ctx, store.EntityQuery{Type: typ}) {
		if err != nil {
			return nil, err
		}
		ids = append(ids, e.ID)
	}
	return ids, nil
}

func collectRelations(ctx context.Context, st store.Store, typ string) ([]*entity.Relation, error) {
	var rels []*entity.Relation
	for r, err := range st.ListRelations(ctx, store.RelationQuery{Type: typ}) {
		if err != nil {
			return nil, err
		}
		rels = append(rels, r)
	}
	return rels, nil
}
