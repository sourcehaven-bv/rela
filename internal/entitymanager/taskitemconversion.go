package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

var (
	ErrUnknownTaskItemConversion = errors.New("entitymanager: no such task item conversion")
	// ErrTaskItemInvalid means the request cannot be applied: bad item text,
	// a missing source face, a source of another type, or caller values for
	// fields the recipe does not leave to the caller.
	ErrTaskItemInvalid = errors.New("entitymanager: task item conversion cannot be applied")
	// ErrTaskItemSourceChanged means a source value the recipe copies changed
	// between the read check and the transaction. Retrying reads it again.
	ErrTaskItemSourceChanged = errors.New("entitymanager: task item conversion source changed; retry")
)

// MaxTaskItemTextRunes bounds the item text a conversion accepts.
const MaxTaskItemTextRunes = 1000

// TaskItemEdgeGate is the relation affordance gate a hand-made relation write
// runs, which lives above this package. A task-item conversion must never
// create an edge the caller could not create by hand.
type TaskItemEdgeGate interface {
	// RelationCreatable reports whether the principal may create a relType
	// edge from `from`. For a recipe with `as: from`, `from` is the entity
	// about to be created.
	RelationCreatable(ctx context.Context, from *entity.Entity, relType string) (bool, error)
}

// TaskItemConverter runs the recipes of `task_item_conversions:`: it creates
// the recipe's target entity from a task item's text and links it to the
// source entity. It does not change the source body; the editor does that.
//
// # Checks
//
// The create and the relation run through [CreateLinked], with its checks:
// the create ACL, the relation ACL, the edge's existing end and the
// relation-type tuple. The converter adds the relation affordance gate, and
// update on the source. That last check is a UX gate, not a security
// boundary: create and relation rights reach the same result by hand.
//
// The item text is the caller's input. It fills the recipe's `{{item.text}}`
// placeholders literally, after the source values, and is never scanned for
// placeholders. Fields it fills, and the values the caller supplies for
// required fields, pass the field write gate like any value the caller
// typed. The recipe's literals and its `{{source.x}}` copies are the
// operator's mapping and are not gated, as for type conversions. A
// `{{source.x}}` the caller cannot read is refused, never copied.
//
// # Atomicity
//
// [CreateLinked] writes the entity and the relation in one store Tx, after
// checking that the source values the recipe copies did not change.
type TaskItemConverter struct {
	m     *Manager
	edges TaskItemEdgeGate
}

// NewTaskItemConverter returns the converter that writes through m.
//
// Nil: rejected — without a manager there is nothing to write through, and
// without an edge gate the converter could create edges a caller cannot.
func NewTaskItemConverter(m *Manager, edges TaskItemEdgeGate) (*TaskItemConverter, error) {
	if m == nil {
		return nil, errors.New("entitymanager: NewTaskItemConverter: manager is required")
	}
	if edges == nil {
		return nil, errors.New("entitymanager: NewTaskItemConverter: edge gate is required")
	}
	return &TaskItemConverter{m: m, edges: edges}, nil
}

// TaskItemRequest invokes a named recipe on one task item of a source entity.
type TaskItemRequest struct {
	Definition, SourceID, SourceFace, Text string
	// Properties holds values for required target fields the recipe leaves
	// empty. Any other key is refused.
	Properties map[string]any
}

// TaskItemOffer is a recipe for a source entity and its advisory verdict.
type TaskItemOffer struct {
	Name, Label, To, Item string
	Allowed               bool
	Reason                string
}

// TaskItemPreview names the required target fields the recipe leaves empty.
// It carries no values, so it cannot read anything the caller cannot.
type TaskItemPreview struct {
	TargetType string
	Required   []string
}

// TaskItemResult reports a committed conversion.
type TaskItemResult struct {
	Definition string
	// Entity is the created entity after automations. Nil: never returned
	// with a nil error.
	Entity *entity.Entity
	// Relation links the source and the created entity.
	Relation *entity.Relation
	// Create carries the create's warnings and automation outcome.
	Create *entity.CreateResult
	// FollowUpErr is a failure of the steps after the commit; see
	// [LinkedResult.FollowUpErr]. The entity and the relation exist.
	FollowUpErr error
}

// taskItemPlan is a request resolved against the stored source.
type taskItemPlan struct {
	name   string
	def    metamodel.TaskItemConversionDef
	source *entity.Entity
	props  map[string]any
	// authored holds the props the caller wrote: the item text's fields and
	// the caller's values for required fields.
	authored map[string]any
	face     entity.Face
	required []string
}

// Definition returns the recipe the converter applies.
func (c *TaskItemConverter) Definition(name string) (metamodel.TaskItemConversionDef, bool) {
	d, ok := c.m.deps.Meta.TaskItemConversions[name]
	return d, ok
}

// Offers returns the recipes whose `from` is the source's type. Allowed is
// advisory: Convert checks again.
func (c *TaskItemConverter) Offers(ctx context.Context, source *entity.Entity) []TaskItemOffer {
	meta := c.m.deps.Meta
	names := make([]string, 0, len(meta.TaskItemConversions))
	for name, d := range meta.TaskItemConversions {
		if source != nil && d.From == source.Type {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}
	out := make([]TaskItemOffer, 0, len(names))
	probe := withAffordanceProbe(ctx)
	// Judged on the row as the caller sees it, as Convert judges it, so an
	// offer marked allowed is one Convert does not refuse for these reasons.
	seen, seenErr := c.seenTaskItemSource(probe, source)
	for _, name := range names {
		d := meta.TaskItemConversions[name]
		o := TaskItemOffer{Name: name, Label: d.Label, To: d.To, Item: d.ItemMode()}
		candidate := &entity.Entity{Type: d.To, Face: entity.Face(d.Face), Properties: map[string]any{}}
		err := seenErr
		if err == nil {
			err = c.refuseHiddenCopies(probe, d, source, seen)
		}
		if err == nil {
			err = c.authorizeTaskItem(probe, d, seen, candidate, nil)
		}
		if err != nil {
			o.Reason = taskItemOfferReason(ctx, name, err)
		} else {
			o.Allowed = true
		}
		out = append(out, o)
	}
	return out
}

// Preview resolves the request without writing and names the required target
// fields still empty. Its authorization is a probe: a denial is returned but
// not audited.
func (c *TaskItemConverter) Preview(ctx context.Context, req TaskItemRequest) (*TaskItemPreview, error) {
	p, err := c.plan(withAffordanceProbe(ctx), req)
	if err != nil {
		return nil, err
	}
	return &TaskItemPreview{TargetType: p.def.To, Required: p.required}, nil
}

// Convert creates the recipe's entity and its relation to the source.
//
// A required field that the recipe and the caller leave empty refuses the
// conversion, although a hand-made create writes it with a warning
// (DEC-HWZHA). The editor asks for those fields at preflight, and an entity
// made from a task item without them is one the user did not mean to
// write.
//
// A failure after the commit is [TaskItemResult.FollowUpErr], not an error:
// the entity and the relation exist, so the editor must still update the
// item. A *[KeptEntityError] reports an entity a failed transaction left
// written.
func (c *TaskItemConverter) Convert(ctx context.Context, req TaskItemRequest) (*TaskItemResult, error) {
	p, err := c.plan(ctx, req)
	if err != nil {
		return nil, err
	}
	if len(p.required) > 0 {
		return nil, newValidationError(requiredErrors(p))
	}
	lc := LinkedCreate{
		Entity:     &entity.Entity{Type: p.def.To, Properties: p.props},
		Options:    entity.CreateOptions{Face: p.face},
		Authored:   p.authored,
		RelateType: p.def.Relation.Type,
		BeforeWrite: func(ctx context.Context, view store.Store) error {
			return requireTaskItemSourceUnchanged(ctx, view, p)
		},
	}
	if p.def.Relation.As == metamodel.TaskItemAsFrom {
		lc.RelateTo = p.source.ID
	} else {
		lc.RelateFrom, lc.RelateFromFace = p.source.ID, p.source.Face
	}
	res, err := CreateLinked(ctx, c.m, lc)
	if err != nil {
		return nil, err
	}
	return &TaskItemResult{
		Definition: p.name, Entity: res.Create.Entity, Relation: res.Relation, Create: res.Create,
		FollowUpErr: res.FollowUpErr,
	}, nil
}

// plan resolves req against the stored source, in this order so a hidden
// source is refused exactly like an absent one: read access, then the type,
// then the text, the fields the recipe copies, the mapping, and
// authorization. The copied-field check comes before the mapping so that no
// later check judges a value the caller cannot read.
func (c *TaskItemConverter) plan(ctx context.Context, req TaskItemRequest) (*taskItemPlan, error) {
	m := c.m
	d, ok := m.deps.Meta.TaskItemConversions[req.Definition]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTaskItemConversion, req.Definition)
	}
	source, err := c.readTaskItemSource(ctx, req)
	if err != nil {
		return nil, err
	}
	if source.Type != d.From {
		return nil, fmt.Errorf("%w: %s is not a %s", ErrTaskItemInvalid, req.SourceID, d.From)
	}
	text, err := validTaskItemText(req.Text)
	if err != nil {
		return nil, err
	}
	seen, err := c.visibleTaskItemSource(ctx, d, source)
	if err != nil {
		return nil, err
	}
	// Values come from the row as the caller sees it, so a reader that
	// strips a hidden field without naming it cannot leak its value.
	props, itemFields, err := taskItemValues(d, seen, text)
	if err != nil {
		return nil, err
	}
	required, err := c.requiredFields(ctx, d, props)
	if err != nil {
		return nil, err
	}
	for key, value := range req.Properties {
		if !slices.Contains(required, key) {
			return nil, fmt.Errorf("%w: field %q is not a required field the recipe leaves empty", ErrTaskItemInvalid, key)
		}
		props[key] = value
	}
	required = slices.DeleteFunc(required, func(key string) bool {
		value, ok := props[key]
		return ok && !metamodel.IsEmptyValue(value)
	})
	authored := make(map[string]any, len(itemFields)+len(req.Properties))
	for _, key := range itemFields {
		authored[key] = props[key]
	}
	for key := range req.Properties {
		authored[key] = props[key]
	}
	p := &taskItemPlan{
		name: req.Definition, def: d, source: source, props: props, authored: authored,
		face: entity.Face(d.Face), required: required,
	}
	candidate := &entity.Entity{Type: d.To, Face: p.face, Properties: props}
	if authErr := c.authorizeTaskItem(ctx, d, seen, candidate, authored); authErr != nil {
		return nil, authErr
	}
	fromType, toType := source.Type, d.To
	if d.Relation.As == metamodel.TaskItemAsFrom {
		fromType, toType = d.To, source.Type
	}
	if vErr := metamodel.NewValidator(m.deps.Meta).ValidateRelation(d.Relation.Type, fromType, toType); vErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrTaskItemInvalid, vErr)
	}
	return p, nil
}

// readTaskItemSource returns the stored source row at req.SourceFace. An
// entity the caller cannot read, at that face or at all, is reported as not
// found.
func (c *TaskItemConverter) readTaskItemSource(ctx context.Context, req TaskItemRequest) (*entity.Entity, error) {
	m := c.m
	family, err := familyRows(ctx, m.deps.Store, req.SourceID)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, newEntityNotFound(req.SourceID)
	}
	gate := faceGateOf(m)
	if err := requireReadableRow(ctx, gate, req.SourceID, family); err != nil {
		return nil, err
	}
	for _, row := range family {
		if row.Face.String() != req.SourceFace {
			continue
		}
		readable, err := readableRows(ctx, gate, []*entity.Entity{row})
		if err != nil {
			return nil, err
		}
		if len(readable) == 0 {
			return nil, newEntityNotFound(req.SourceID)
		}
		return row, nil
	}
	if req.SourceFace == "" {
		return nil, fmt.Errorf("%w: source_face is required for %s", ErrTaskItemInvalid, req.SourceID)
	}
	return nil, newEntityNotFound(entity.FormatStateRef(req.SourceID, entity.Face(req.SourceFace)))
}

// visibleTaskItemSource returns source as the caller sees it, and refuses a
// recipe that copies a field hidden from the caller.
func (c *TaskItemConverter) visibleTaskItemSource(
	ctx context.Context, d metamodel.TaskItemConversionDef, source *entity.Entity,
) (*entity.Entity, error) {
	seen, err := c.seenTaskItemSource(ctx, source)
	if err != nil {
		return nil, err
	}
	if err := c.refuseHiddenCopies(ctx, d, source, seen); err != nil {
		return nil, err
	}
	return seen, nil
}

// seenTaskItemSource reads source as the caller sees it. A handle without a
// copy reader, or one that bypasses the ACL, sees the stored row.
func (c *TaskItemConverter) seenTaskItemSource(ctx context.Context, source *entity.Entity) (*entity.Entity, error) {
	m := c.m
	reader := m.deps.CopyVisibility
	if reader == nil || m.bypassACL || m.cascadeWrite {
		return source, nil
	}
	seen, ok, err := reader.Get(ctx, source.Type, source.ID, source.Face)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, newEntityNotFound(source.ID)
	}
	return seen, nil
}

// refuseHiddenCopies refuses a recipe that copies a field of source hidden
// in seen. The refusal depends on the hidden set, never on the hidden value,
// so it tells the caller nothing a read of the source does not: a read
// serves the same names as `_redacted`.
func (c *TaskItemConverter) refuseHiddenCopies(
	ctx context.Context, d metamodel.TaskItemConversionDef, source, seen *entity.Entity,
) error {
	for _, field := range taskItemSourceFields(d) {
		if seen.IsRedacted(field) {
			return c.deny(ctx, acl.Decision{
				RuleKind: "field-read", RuleID: field,
				Reason: fmt.Sprintf("you cannot read source field %q, which the recipe copies into a new %s", field, d.To),
				// The source is the subject: the caller passed its read gate,
				// and it is the entity an auditor needs to see probed.
			}, acl.WriteRequest{Op: acl.OpCreate, Subject: acl.NewEntitySubject(source.Type, source.ID, source.Face)})
		}
	}
	return nil
}

// deny refuses the conversion with decision. Unless ctx is an affordance
// probe it records the refusal as a denied write of req, as
// authorizeAndAudit does for an ACL refusal.
func (c *TaskItemConverter) deny(ctx context.Context, decision acl.Decision, req acl.WriteRequest) error {
	if !isAffordanceProbe(ctx) {
		c.m.deps.Audit.Record(deniedWriteRecord(ctx, decision, req))
	}
	return &acl.ForbiddenError{Decision: decision}
}

// taskItemOfferReason is the reason an offer is not allowed, as the caller
// may see it: a refusal's own reason, or a fixed text for any other error,
// which is logged instead.
func taskItemOfferReason(ctx context.Context, name string, err error) string {
	if forbidden, ok := errors.AsType[*acl.ForbiddenError](err); ok {
		return forbidden.Decision.Reason
	}
	switch ClassifyCreateError(err) {
	case CreateErrForbidden, CreateErrFace:
		return err.Error()
	case CreateErrUnknown, CreateErrConflict, CreateErrInvalid:
	}
	slog.WarnContext(ctx, "entitymanager: task item conversion offer failed", "conversion", name, "error", err)
	return "This conversion is not available."
}

// authorizeTaskItem runs the create checks for candidate (gated holds the
// fields the caller authored), the relation checks, and update on the
// source. source is the row as the caller sees it, so the relation gate
// judges no hidden value. Offers passes a nil gated set: it has no item
// text yet.
func (c *TaskItemConverter) authorizeTaskItem(
	ctx context.Context, d metamodel.TaskItemConversionDef, source, candidate *entity.Entity, gated map[string]any,
) error {
	m := c.m
	if source == nil {
		return newEntityNotFound("")
	}
	if err := prepareCreate(ctx, m, candidate, entity.CreateOptions{Face: candidate.Face}, gated); err != nil {
		return err
	}
	fromType, fromID, tail := taskItemRelationFrom(m.deps.Meta, d, source)
	if err := m.deps.requireRelationFaceFor(d.Relation.Type, fromType, tail); err != nil {
		return err
	}
	if err := m.authorizeAndAudit(ctx,
		RelationCreateRequest(m.deps.Meta, d.Relation.Type, fromType, fromID, tail)); err != nil {
		return err
	}
	gateFrom := source
	if d.Relation.As == metamodel.TaskItemAsFrom {
		gateFrom = candidate
	}
	creatable, err := c.edges.RelationCreatable(ctx, gateFrom, d.Relation.Type)
	if err != nil {
		return fmt.Errorf("entitymanager: task item conversion: relation gate: %w", err)
	}
	if !creatable {
		return c.deny(ctx, acl.Decision{
			RuleKind: "relation-affordance", RuleID: d.Relation.Type,
			Reason: fmt.Sprintf("you may not create a %s relation here", d.Relation.Type),
		}, RelationCreateRequest(m.deps.Meta, d.Relation.Type, fromType, fromID, tail))
	}
	return m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpUpdate,
		Subject: acl.NewEntitySubject(source.Type, source.ID, source.Face),
	})
}

// taskItemRelationFrom returns the relation's from type, from id and tail.
// The tail of a content-scoped relation is the face of its from end: the
// source face for `as: to`, the recipe face for `as: from`. An
// identity-scoped relation has the implicit tail. The id is empty for
// `as: from`, where the from end does not exist yet, as for a create.
func taskItemRelationFrom(
	meta *metamodel.Metamodel, d metamodel.TaskItemConversionDef, source *entity.Entity,
) (fromType, fromID string, face entity.Face) {
	fromType, fromID, face = source.Type, source.ID, source.Face
	if d.Relation.As == metamodel.TaskItemAsFrom {
		fromType, fromID, face = d.To, "", entity.Face(d.Face)
	}
	if !metamodel.IsContentScoped(meta, d.Relation.Type) {
		face = entity.ImplicitFace
	}
	return fromType, fromID, face
}

// validTaskItemText trims text and checks it is one line of at most
// [MaxTaskItemTextRunes] runes, without control characters. Bidi controls
// are refused too: they make a title read differently from its bytes.
// Other format characters stay, since emoji sequences and some scripts
// need the zero-width joiners.
func validTaskItemText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("%w: item text is empty", ErrTaskItemInvalid)
	}
	if !utf8.ValidString(text) {
		return "", fmt.Errorf("%w: item text is not valid UTF-8", ErrTaskItemInvalid)
	}
	if utf8.RuneCountInString(text) > MaxTaskItemTextRunes {
		return "", fmt.Errorf("%w: item text is longer than %d characters", ErrTaskItemInvalid, MaxTaskItemTextRunes)
	}
	for _, r := range text {
		if unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r) || r == '\u2028' || r == '\u2029' {
			return "", fmt.Errorf("%w: item text must be a single line without control characters", ErrTaskItemInvalid)
		}
	}
	return text, nil
}

// taskItemValues expands the recipe's field templates. A template that is
// exactly one `{{source.x}}` copies the typed value; otherwise the pieces are
// joined as text. The item text is inserted literally and never re-scanned.
// It also returns the fields whose template uses the item text.
func taskItemValues(
	d metamodel.TaskItemConversionDef, source *entity.Entity, text string,
) (props map[string]any, itemFields []string, err error) {
	props = make(map[string]any, len(d.Fields))
	for key, template := range d.Fields {
		tokens, err := metamodel.ParseTaskItemTemplate(template)
		if err != nil {
			return nil, nil, fmt.Errorf("%w: fields.%s: %w", ErrTaskItemInvalid, key, err)
		}
		if metamodel.UsesItemText(tokens) {
			itemFields = append(itemFields, key)
		}
		if len(tokens) == 1 && tokens[0].Kind == metamodel.TokenSource {
			if value, ok := source.Properties[tokens[0].Value]; ok && !metamodel.IsEmptyValue(value) {
				props[key] = value
			}
			continue
		}
		var b strings.Builder
		for _, t := range tokens {
			switch t.Kind {
			case metamodel.TokenLiteral:
				b.WriteString(t.Value)
			case metamodel.TokenItemText:
				b.WriteString(text)
			case metamodel.TokenSource:
				if value, ok := source.Properties[t.Value]; ok && !metamodel.IsEmptyValue(value) {
					b.WriteString(taskItemValueText(value))
				}
			}
		}
		if b.Len() > 0 {
			props[key] = b.String()
		}
	}
	sort.Strings(itemFields)
	return props, itemFields, nil
}

// taskItemValueText renders a source value inside a text template: a date
// as YYYY-MM-DD and a list as its items joined by ", ".
func taskItemValueText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case time.Time:
		return v.Format(time.DateOnly)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if !metamodel.IsEmptyValue(item) {
				parts = append(parts, taskItemValueText(item))
			}
		}
		return strings.Join(parts, ", ")
	case []string:
		return strings.Join(v, ", ")
	case float64:
		// fmt.Sprint writes 1e+06 for a whole number of a million.
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprint(value)
}

// taskItemSourceFields returns the source properties the recipe reads.
func taskItemSourceFields(d metamodel.TaskItemConversionDef) []string {
	var fields []string
	for _, template := range d.Fields {
		tokens, err := metamodel.ParseTaskItemTemplate(template)
		if err != nil {
			continue
		}
		for _, t := range tokens {
			if t.Kind == metamodel.TokenSource {
				fields = append(fields, t.Value)
			}
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(fields)))
}

// requiredFields returns the required, non-computed target fields that are
// still empty once the recipe's values and the entity template's defaults
// apply, sorted. It builds a dry-run candidate, so it mints no id.
func (c *TaskItemConverter) requiredFields(
	ctx context.Context, d metamodel.TaskItemConversionDef, props map[string]any,
) ([]string, error) {
	m := c.m
	candidate, _, err := buildCandidateEntity(ctx, m.deps, d.To, createCoreOpts{
		Properties:       maps.Clone(props),
		Face:             entity.Face(d.Face),
		SkipIDGeneration: true,
	})
	if err != nil {
		return nil, err
	}
	target, _ := m.deps.Meta.GetEntityDef(d.To)
	var required []string
	for key, property := range target.Properties {
		if !property.Required || property.Computed != "" {
			continue
		}
		if value, ok := candidate.Properties[key]; !ok || metamodel.IsEmptyValue(value) {
			required = append(required, key)
		}
	}
	sort.Strings(required)
	return required, nil
}

func requiredErrors(p *taskItemPlan) []*metamodel.ValidationError {
	out := make([]*metamodel.ValidationError, 0, len(p.required))
	for _, key := range p.required {
		out = append(out, &metamodel.ValidationError{
			Type: metamodel.ValidationErrorRequired, Property: key, Message: key + " is required",
		})
	}
	return out
}

// requireTaskItemSourceUnchanged refuses the Tx when a source value the recipe
// copies differs from the one plan checked: field visibility can depend on
// values, so a change in between could copy a value nobody checked.
func requireTaskItemSourceUnchanged(ctx context.Context, view store.Store, p *taskItemPlan) error {
	family, err := familyRows(ctx, view, p.source.ID)
	if err != nil {
		return err
	}
	for _, row := range family {
		if row.Face != p.source.Face {
			continue
		}
		if row.Type != p.source.Type {
			return ErrTaskItemSourceChanged
		}
		for _, field := range taskItemSourceFields(p.def) {
			if !reflect.DeepEqual(row.Properties[field], p.source.Properties[field]) {
				return ErrTaskItemSourceChanged
			}
		}
		return nil
	}
	return ErrTaskItemSourceChanged
}
