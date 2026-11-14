package mapimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RawReader lists stored rows without the read gate, so the plan can find
// every row a key names, including rows the principal cannot read. Nothing
// it returns is named or compared before [ReadGate] admits it, and values
// are taken from the gated row.
type RawReader interface {
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// ReadGate reads a row as the principal on ctx may see it. A row the
// principal cannot read is [store.ErrNotFound], like an absent one.
type ReadGate interface {
	GetAddress(ctx context.Context, addr string) (*entity.Entity, error)
}

// Deps are the collaborators of [Build].
type Deps struct {
	Meta  *metamodel.Metamodel
	Store RawReader
	Gate  ReadGate
	// Files loads hook scripts. Nil is accepted when no mapping has a hook.
	Files ProjectFiles
}

// Plan is what an import would change. Build computes it without writing;
// [Apply] carries it out.
type Plan struct {
	Mapping         string           `json:"mapping"`
	Creates         []EntityChange   `json:"creates"`
	Updates         []EntityChange   `json:"updates"`
	Unchanged       int              `json:"unchanged"`
	Skipped         int              `json:"skipped"`
	RelationAdds    []RelationChange `json:"relation_adds"`
	RelationRemoves []RelationChange `json:"relation_removes"`
	// Stale lists stored relations the source no longer names, on relation
	// mappings without prune. They stay.
	Stale []RelationChange `json:"stale"`
	// Missing lists stored entities in an import's scope that the source no
	// longer holds. They stay; the import never deletes an entity.
	Missing []MissingEntity `json:"missing"`
	Errors  []RecordError   `json:"errors"`

	creates []*createOp
	patches []*patchOp
	adds    []*relOp
	removes []entity.RelationKey
}

// EntityChange is one entity to create or update.
type EntityChange struct {
	Mapping string `json:"mapping"`
	Type    string `json:"type"`
	Key     string `json:"key"`
	// ID is empty for a create whose id the manager generates.
	ID             string           `json:"id,omitempty"`
	Properties     []PropertyChange `json:"properties,omitempty"`
	ContentChanged bool             `json:"content_changed,omitempty"`
}

// PropertyChange is one property an update sets or removes.
type PropertyChange struct {
	Name    string `json:"name"`
	Old     any    `json:"old,omitempty"`
	New     any    `json:"new,omitempty"`
	Cleared bool   `json:"cleared,omitempty"`
}

// RelationChange is one relation. An endpoint the run creates is shown as
// "(new <type> <key>)" when its id is not known yet.
type RelationChange struct {
	Type string `json:"type"`
	From string `json:"from"`
	To   string `json:"to"`
}

// MissingEntity is a stored entity in scope that the source no longer holds.
type MissingEntity struct {
	Mapping string `json:"mapping"`
	Type    string `json:"type"`
	ID      string `json:"id"`
	Key     string `json:"key"`
}

// RecordError is a problem with one record, located in the source.
type RecordError struct {
	Mapping  string `json:"mapping"`
	Location string `json:"location"`
	Message  string `json:"message"`
}

// HasChanges reports whether applying the plan would write anything.
func (p *Plan) HasChanges() bool {
	return len(p.creates)+len(p.patches)+len(p.adds)+len(p.removes) > 0
}

type createOp struct {
	label string
	e     *entity.Entity
	opts  entity.CreateOptions
	// id is set from the start when the mapping templates it, and from the
	// manager's result otherwise.
	id string
}

type patchOp struct {
	id    string
	patch entity.Patch
}

// endpoint is one side of a relation to add: a stored id or an entity the
// run creates.
type endpoint struct {
	id  string
	new *createOp
}

func (e endpoint) label() string {
	if e.new != nil && e.id == "" {
		return e.new.label
	}
	return e.endID()
}

func (e endpoint) endID() string {
	if e.new != nil && e.new.id != "" {
		return e.new.id
	}
	return e.id
}

type relOp struct {
	typ      string
	from, to endpoint
}

// typeIndex holds the stored rows of one type.
type typeIndex struct {
	rows []*entity.Entity
	byID map[string]*entity.Entity
}

// mappingState is one entity mapping's records and their matched rows.
type mappingState struct {
	em     *EntityMapping
	def    *metamodel.EntityDef
	drafts []*draft
	byKey  map[string]*draft
	target map[*draft]endpoint
	// inScope holds the stored rows in scope by key; outScope the other
	// rows of the type by key.
	inScope, outScope map[string][]*entity.Entity
	seen              map[string]bool // keys the source holds, errored ones included
	// matched is each live draft's stored row as the principal reads it,
	// nil for a create.
	matched map[*draft]*entity.Entity
}

// live reports whether d is still planned: it has no record error.
func (st *mappingState) live(d *draft) bool { return st.byKey[d.key] == d }

// drop leaves d out of the plan after a record error.
func (st *mappingState) drop(d *draft) {
	if st.live(d) {
		delete(st.byKey, d.key)
	}
}

// ident lists the properties that decide which stored row a key names: the
// key and the scope. A row that hides one of them from the principal is
// treated as unreadable, so the plan never reveals their values.
func (st *mappingState) ident() []string {
	out := make([]string, 0, len(st.em.Scope)+1)
	if st.em.Key != "" {
		out = append(out, st.em.Key)
	}
	return append(out, sortedKeys(st.em.Scope)...)
}

type builder struct {
	d       Deps
	m       *Mapping
	plan    *Plan
	types   map[string]*typeIndex
	states  map[string]*mappingState
	gated   map[string]*entity.Entity // address → row as the principal reads it; nil when hidden
	targets map[*RelationMapping]*relTarget
	rels    map[string][]*entity.Relation // relation type → stored relations
	addSeen map[string]bool
}

// Build plans the import of root under m. It reads stored rows and writes
// nothing. A problem with one record goes in Plan.Errors; an error return
// means the plan could not be made at all.
func Build(ctx context.Context, d Deps, m *Mapping, root *Node) (*Plan, error) {
	switch {
	case d.Meta == nil:
		return nil, errors.New("mapimport: Deps.Meta is required")
	case d.Store == nil:
		return nil, errors.New("mapimport: Deps.Store is required")
	case d.Gate == nil:
		return nil, errors.New("mapimport: Deps.Gate is required")
	case m == nil || root == nil:
		return nil, errors.New("mapimport: a mapping and a document are required")
	}
	for _, name := range m.Names() {
		if em := m.Entities[name]; em == nil || em.selectPath == nil {
			return nil, fmt.Errorf("mapimport: mapping %s was not loaded with Parse", name)
		}
	}
	b := &builder{
		d: d, m: m,
		plan:    &Plan{Mapping: m.Name},
		types:   map[string]*typeIndex{},
		states:  map[string]*mappingState{},
		gated:   map[string]*entity.Entity{},
		targets: map[*RelationMapping]*relTarget{},
		rels:    map[string][]*entity.Relation{},
		addSeen: map[string]bool{},
	}
	for _, name := range m.Names() {
		if err := b.derive(ctx, name, root); err != nil {
			return nil, err
		}
	}
	for _, name := range m.Names() {
		if err := b.match(ctx, b.states[name]); err != nil {
			return nil, err
		}
	}
	// Every record error is known before any write is planned, so a record
	// with an error is left out whole.
	if err := b.settle(ctx); err != nil {
		return nil, err
	}
	for _, name := range m.Names() {
		b.emit(b.states[name])
	}
	for _, name := range m.Names() {
		if err := b.relations(ctx, b.states[name]); err != nil {
			return nil, err
		}
	}
	for _, name := range m.Names() {
		if err := b.missing(ctx, b.states[name]); err != nil {
			return nil, err
		}
	}
	return b.plan, nil
}

func (b *builder) fail(em *EntityMapping, loc, format string, args ...any) {
	b.plan.Errors = append(b.plan.Errors, RecordError{
		Mapping: em.name, Location: loc, Message: fmt.Sprintf(format, args...),
	})
}

// derive turns each selected record into a draft.
func (b *builder) derive(ctx context.Context, name string, root *Node) error {
	em := b.m.Entities[name]
	def, ok := b.d.Meta.GetEntityDef(em.Type)
	if !ok {
		return fmt.Errorf("mapimport: type %q is not in the schema", em.Type)
	}
	st := &mappingState{
		em: em, def: def, byKey: map[string]*draft{}, target: map[*draft]endpoint{}, seen: map[string]bool{},
		matched: map[*draft]*entity.Entity{},
	}
	b.states[name] = st
	records := em.selectPath.Eval(root, root)
	if len(records) > b.m.maxRecords() {
		return fmt.Errorf("%s: select %q yields %d records; max_records is %d",
			name, em.Select, len(records), b.m.maxRecords())
	}
	var hook *hookRunner
	if em.Hook != nil {
		h, err := newHook(ctx, b.d.Files, em.Hook)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		defer h.close()
		hook = h
	}
	for _, rec := range records {
		if err := ctx.Err(); err != nil {
			return err
		}
		d, rerr := derive(em, def, rec, root, hook)
		if rerr != nil {
			b.fail(em, rerr.loc, "%s", rerr.msg)
			continue
		}
		if d == nil {
			b.plan.Skipped++
			continue
		}
		st.seen[d.key] = true
		if first := st.byKey[d.key]; first != nil {
			b.fail(em, d.loc, "key %q is also used by the record at %s", d.key, first.loc)
			continue
		}
		st.byKey[d.key] = d
		st.drafts = append(st.drafts, d)
	}
	return nil
}

// index lists the stored rows of typ once.
func (b *builder) index(ctx context.Context, typ string) (*typeIndex, error) {
	if ix := b.types[typ]; ix != nil {
		return ix, nil
	}
	ix := &typeIndex{byID: map[string]*entity.Entity{}}
	for e, err := range b.d.Store.ListEntities(ctx, store.EntityQuery{Type: typ, Faces: store.AllFaces()}) {
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", typ, err)
		}
		ix.rows = append(ix.rows, e)
		ix.byID[e.ID] = e
	}
	b.types[typ] = ix
	return ix, nil
}

// read returns the row as the principal reads it through the gate, with
// hidden property values stripped and named in Redacted. It is nil when the
// principal may not read the row.
func (b *builder) read(ctx context.Context, e *entity.Entity) (*entity.Entity, error) {
	addr := e.Ref().String()
	if g, done := b.gated[addr]; done {
		return g, nil
	}
	g, err := b.d.Gate.GetAddress(ctx, addr)
	switch {
	case err == nil:
	case errors.Is(err, store.ErrNotFound):
		g = nil
	default:
		return nil, fmt.Errorf("read gate: %w", err)
	}
	b.gated[addr] = g
	return g, nil
}

// usable is read, and reports ok false also when the row hides one of
// ident.
func (b *builder) usable(ctx context.Context, e *entity.Entity, ident []string) (*entity.Entity, bool, error) {
	g, err := b.read(ctx, e)
	if err != nil || g == nil {
		return nil, false, err
	}
	for _, name := range ident {
		if slices.Contains(g.Redacted, name) {
			return nil, false, nil
		}
	}
	return g, true, nil
}

func (st *mappingState) storedKey(e *entity.Entity) string {
	if st.em.Key == "" {
		return e.ID
	}
	return keyText(st.def.Properties[st.em.Key], e.Properties[st.em.Key])
}

func (st *mappingState) inScopeRow(e *entity.Entity) bool {
	for name, want := range st.em.Scope {
		if normalize(e.Properties[name]) != normalize(scopeValue(st.def, name, want)) {
			return false
		}
	}
	return true
}

// scopeValue is a scope constant in its stored form. Parse checked that it
// converts.
func scopeValue(def *metamodel.EntityDef, name, v string) any {
	c, err := coerce(def.Properties[name], v)
	if err != nil {
		return v
	}
	return c
}

// match pairs each draft with its stored row. A draft that cannot be
// matched is dropped with a record error.
func (b *builder) match(ctx context.Context, st *mappingState) error {
	ix, err := b.index(ctx, st.em.Type)
	if err != nil {
		return err
	}
	st.inScope, st.outScope = map[string][]*entity.Entity{}, map[string][]*entity.Entity{}
	for _, r := range ix.rows {
		k := st.storedKey(r)
		if k == "" {
			continue
		}
		if st.inScopeRow(r) {
			st.inScope[k] = append(st.inScope[k], r)
		} else {
			st.outScope[k] = append(st.outScope[k], r)
		}
	}
	for _, d := range st.drafts {
		if !st.live(d) {
			continue
		}
		g, rerr, err := b.matchOne(ctx, st, ix, d)
		if err != nil {
			return err
		}
		if rerr == "" && g != nil {
			rerr = hiddenTarget(st, d, g)
		}
		if rerr != "" {
			b.fail(st.em, d.loc, "%s", rerr)
			st.drop(d)
			continue
		}
		st.matched[d] = g
	}
	return nil
}

// matchOne finds d's stored row as the principal reads it, nil for a new
// entity. A non-empty rerr is a record error.
func (b *builder) matchOne(
	ctx context.Context, st *mappingState, ix *typeIndex, d *draft,
) (*entity.Entity, string, error) {
	ident := st.ident()
	rows := st.inScope[d.key]
	if len(rows) > 1 {
		return nil, fmt.Sprintf("key %q matches several stored entities", d.key), nil
	}
	if len(rows) == 1 {
		g, ok, err := b.usable(ctx, rows[0], ident)
		if err != nil || !ok {
			return nil, heldHidden(d.key), err
		}
		return g, "", nil
	}
	if other := st.outScope[d.key]; len(other) > 0 {
		g, ok, err := b.usable(ctx, other[0], ident)
		if err != nil || !ok {
			return nil, heldHidden(d.key), err
		}
		return nil, fmt.Sprintf("key %q is held by %s, outside this import's scope", d.key, g.ID), nil
	}
	if d.id != "" {
		if e := ix.byID[d.id]; e != nil {
			g, err := b.read(ctx, e)
			if err != nil || g == nil {
				return nil, fmt.Sprintf("id %q is held by an entity you cannot read", d.id), err
			}
			return nil, fmt.Sprintf("id %q is already used by an entity with another key", d.id), nil
		}
	}
	return nil, "", nil
}

func heldHidden(key string) string {
	return fmt.Sprintf("key %q is held by an entity you cannot read", key)
}

// hiddenTarget returns a record error when d would compare or write a
// property whose value g hides from the principal. Comparing it would show
// the hidden value, or whether the source matches it.
func hiddenTarget(st *mappingState, d *draft, g *entity.Entity) string {
	if len(g.Redacted) == 0 {
		return ""
	}
	names := sortedKeys(d.props)
	for _, name := range d.absent {
		if v := st.em.Properties[name]; v != nil && v.ClearMissing {
			names = append(names, name)
		}
	}
	for _, name := range names {
		if slices.Contains(g.Redacted, name) {
			return fmt.Sprintf("property %s of %s is hidden from you, so the import cannot compare it", name, g.ID)
		}
	}
	return ""
}

// settle drops every draft whose relations name a target that cannot be
// resolved. Dropping a draft can break a relation to it, so it repeats until
// nothing changes.
func (b *builder) settle(ctx context.Context) error {
	for changed := true; changed; {
		changed = false
		for _, name := range b.m.Names() {
			st := b.states[name]
			for _, d := range st.drafts {
				if !st.live(d) {
					continue
				}
				msg, err := b.relationError(ctx, st, d)
				if err != nil {
					return err
				}
				if msg != "" {
					b.fail(st.em, d.loc, "%s", msg)
					st.drop(d)
					changed = true
				}
			}
		}
	}
	return nil
}

// relationError returns the first record error among d's relations.
func (b *builder) relationError(ctx context.Context, st *mappingState, d *draft) (string, error) {
	for _, rm := range st.em.Relations {
		if rm.Optional {
			continue
		}
		t, err := b.targetFor(ctx, rm)
		if err != nil {
			return "", err
		}
		for _, key := range d.rels[rm.Name] {
			_, msg, err := t.resolve(ctx, key)
			if err != nil {
				return "", err
			}
			if msg != "" {
				return fmt.Sprintf("relations.%s: %s", rm.Name, msg), nil
			}
		}
	}
	return "", nil
}

// emit plans the create or update of each live draft.
func (b *builder) emit(st *mappingState) {
	for _, d := range st.drafts {
		if !st.live(d) {
			continue
		}
		g := st.matched[d]
		if g == nil {
			st.target[d] = endpoint{id: d.id, new: b.planCreate(st, d)}
			continue
		}
		st.target[d] = endpoint{id: g.ID}
		b.planUpdate(st, d, g)
	}
}

func (b *builder) planCreate(st *mappingState, d *draft) *createOp {
	props := maps.Clone(d.props)
	for k, v := range st.em.Scope {
		props[k] = scopeValue(st.def, k, v)
	}
	e := &entity.Entity{Type: st.em.Type, Properties: props}
	if d.content != nil {
		e.Content = *d.content
	}
	label := fmt.Sprintf("(new %s %s)", st.em.Type, d.key)
	op := &createOp{
		label: label, e: e, id: d.id,
		opts: entity.CreateOptions{ID: d.id, WriteExternalRefs: namesExternalRef(st.def, props)},
	}
	b.plan.creates = append(b.plan.creates, op)
	change := EntityChange{
		Mapping: st.em.name, Type: st.em.Type, Key: d.key, ID: d.id, ContentChanged: d.content != nil,
	}
	for _, name := range sortedKeys(props) {
		change.Properties = append(change.Properties, PropertyChange{Name: name, New: props[name]})
	}
	b.plan.Creates = append(b.plan.Creates, change)
	return op
}

// planUpdate diffs d against g, the stored row as the principal reads it.
// match has refused a draft that names a property g hides, so every value
// compared here is one the principal may see.
func (b *builder) planUpdate(st *mappingState, d *draft, g *entity.Entity) {
	change := EntityChange{Mapping: st.em.name, Type: st.em.Type, Key: d.key, ID: g.ID}
	patch := entity.Patch{Properties: map[string]any{}}
	for _, name := range sortedKeys(d.props) {
		pd := st.def.Properties[name]
		if sameValue(pd, g.Properties[name], d.props[name]) {
			continue
		}
		patch.Properties[name] = d.props[name]
		change.Properties = append(change.Properties,
			PropertyChange{Name: name, Old: g.Properties[name], New: d.props[name]})
	}
	for _, name := range d.absent {
		v := st.em.Properties[name]
		if v == nil || !v.ClearMissing || metamodel.IsEmptyValue(g.Properties[name]) {
			continue
		}
		patch.MetaUnset = append(patch.MetaUnset, name)
		change.Properties = append(change.Properties,
			PropertyChange{Name: name, Old: g.Properties[name], Cleared: true})
	}
	if d.content != nil && trimBody(*d.content) != trimBody(g.Content) {
		body := *d.content
		patch.Content = &body
		change.ContentChanged = true
	}
	if patch.IsEmpty() {
		b.plan.Unchanged++
		return
	}
	changed := maps.Clone(patch.Properties)
	for _, k := range patch.MetaUnset {
		changed[k] = nil
	}
	patch.WriteExternalRefs = namesExternalRef(st.def, changed)
	b.plan.patches = append(b.plan.patches, &patchOp{id: g.ID, patch: patch})
	b.plan.Updates = append(b.plan.Updates, change)
}

func namesExternalRef(def *metamodel.EntityDef, props map[string]any) bool {
	for name := range props {
		if def.Properties[name].Type == metamodel.PropertyTypeExternalRef {
			return true
		}
	}
	return false
}

// storedRels lists the stored relations of typ once.
func (b *builder) storedRels(ctx context.Context, typ string) ([]*entity.Relation, error) {
	if rs, ok := b.rels[typ]; ok {
		return rs, nil
	}
	rs := []*entity.Relation{}
	for r, err := range b.d.Store.ListRelations(ctx, store.RelationQuery{Type: typ}) {
		if err != nil {
			return nil, fmt.Errorf("list %s relations: %w", typ, err)
		}
		rs = append(rs, r)
	}
	b.rels[typ] = rs
	return rs, nil
}

// relations plans the relations of each matched or created draft.
func (b *builder) relations(ctx context.Context, st *mappingState) error {
	for _, rm := range st.em.Relations {
		existing, err := b.storedRels(ctx, rm.Type)
		if err != nil {
			return err
		}
		t, err := b.targetFor(ctx, rm)
		if err != nil {
			return err
		}
		out := rm.out()
		// Index the stored edges by this mapping's end.
		byOwn := map[string][]*entity.Relation{}
		for _, r := range existing {
			own := r.From
			if !out {
				own = r.To
			}
			byOwn[own] = append(byOwn[own], r)
		}
		for _, d := range st.drafts {
			self, ok := st.target[d]
			if !ok {
				continue
			}
			adds, want, err := b.wanted(ctx, rm, d, self, t)
			if err != nil {
				return err
			}
			var stored []*entity.Relation
			if self.new == nil {
				stored = byOwn[self.id]
			}
			have, err := b.unwanted(ctx, rm, stored, want, t)
			if err != nil {
				return err
			}
			b.addRelations(adds, have, out)
		}
	}
	return nil
}

// relTarget is how one relation mapping finds its targets.
type relTarget struct {
	// resolve finds the endpoint for a target key. A non-empty message is a
	// record error. Before emit, the endpoint of a draft in this run is
	// zero; only the message counts then.
	resolve func(ctx context.Context, key string) (ep endpoint, msg string, err error)
	// candidate returns the stored row id when it is one of this mapping's
	// targets, for stale and pruned relations.
	candidate func(id string) *entity.Entity
	// ident lists the target properties a stored row must not hide.
	ident []string
}

// targetFor returns rm's target resolver, built once per run.
func (b *builder) targetFor(ctx context.Context, rm *RelationMapping) (*relTarget, error) {
	if t := b.targets[rm]; t != nil {
		return t, nil
	}
	var t *relTarget
	var err error
	if rm.Target != "" {
		t, err = b.mappingTarget(ctx, rm)
	} else {
		t, err = b.typeTarget(ctx, rm)
	}
	if err != nil {
		return nil, err
	}
	b.targets[rm] = t
	return t, nil
}

// targetKey brings a source key to the form keys are compared in, as the
// target's key property stores it.
func targetKey(def *metamodel.EntityDef, prop, key string) string {
	if prop == "" {
		return key
	}
	pd := def.Properties[prop]
	c, err := coerce(pd, key)
	if err != nil {
		return key
	}
	return keyText(pd, c)
}

// mappingTarget resolves keys against another entity mapping: its records
// first, then its stored rows in scope.
func (b *builder) mappingTarget(ctx context.Context, rm *RelationMapping) (*relTarget, error) {
	ts := b.states[rm.Target]
	ix, err := b.index(ctx, ts.em.Type)
	if err != nil {
		return nil, err
	}
	ident := ts.ident()
	resolve := func(ctx context.Context, key string) (endpoint, string, error) {
		key = targetKey(ts.def, ts.em.Key, key)
		if td := ts.byKey[key]; td != nil {
			return ts.target[td], "", nil
		}
		rows := ts.inScope[key]
		if ts.seen[key] && len(rows) == 0 {
			return endpoint{}, fmt.Sprintf("target %s %q has record errors", rm.Target, key), nil
		}
		return b.single(ctx, rows, key, fmt.Sprintf("no %s with key %q", rm.Target, key), ident)
	}
	candidate := func(id string) *entity.Entity {
		if e := ix.byID[id]; e != nil && ts.inScopeRow(e) {
			return e
		}
		return nil
	}
	return &relTarget{resolve: resolve, candidate: candidate, ident: ident}, nil
}

// typeTarget resolves keys against stored rows of a type the file does not
// import, by a unique or external_ref property.
func (b *builder) typeTarget(ctx context.Context, rm *RelationMapping) (*relTarget, error) {
	ix, err := b.index(ctx, rm.TargetType)
	if err != nil {
		return nil, err
	}
	def, _ := b.d.Meta.GetEntityDef(rm.TargetType)
	pd := def.Properties[rm.TargetKey]
	byKey := map[string][]*entity.Entity{}
	for _, r := range ix.rows {
		if k := keyText(pd, r.Properties[rm.TargetKey]); k != "" {
			byKey[k] = append(byKey[k], r)
		}
	}
	ident := []string{rm.TargetKey}
	resolve := func(ctx context.Context, key string) (endpoint, string, error) {
		key = targetKey(def, rm.TargetKey, key)
		return b.single(ctx, byKey[key], key,
			fmt.Sprintf("no %s with %s %q", rm.TargetType, rm.TargetKey, key), ident)
	}
	candidate := func(id string) *entity.Entity { return ix.byID[id] }
	return &relTarget{resolve: resolve, candidate: candidate, ident: ident}, nil
}

// single picks the one usable row a target key names. none is the message
// for no row.
func (b *builder) single(
	ctx context.Context, rows []*entity.Entity, key, none string, ident []string,
) (endpoint, string, error) {
	switch {
	case len(rows) == 0:
		return endpoint{}, none, nil
	case len(rows) > 1:
		return endpoint{}, fmt.Sprintf("key %q matches several stored %s entities", key, rows[0].Type), nil
	}
	g, ok, err := b.usable(ctx, rows[0], ident)
	if err != nil || !ok {
		return endpoint{}, heldHidden(key), err
	}
	return endpoint{id: g.ID}, "", nil
}

// wanted resolves d's target keys for rm into the relations the source
// names. want holds the stored target ids among them. settle has dropped
// every draft with an unresolved key, so a message here is an optional
// relation's and is skipped.
func (b *builder) wanted(
	ctx context.Context, rm *RelationMapping, d *draft, self endpoint, t *relTarget,
) (adds []*relOp, want map[string]bool, err error) {
	want = map[string]bool{}
	for _, key := range d.rels[rm.Name] {
		ep, msg, err := t.resolve(ctx, key)
		if err != nil {
			return nil, nil, err
		}
		if msg != "" {
			continue
		}
		if ep.new == nil {
			want[ep.id] = true
		}
		op := &relOp{typ: rm.Type, from: self, to: ep}
		if !rm.out() {
			op.from, op.to = ep, self
		}
		adds = append(adds, op)
	}
	return adds, want, nil
}

// unwanted plans the stored relations the source no longer names: removes
// under prune, stale reports otherwise. It returns the ids the stored
// relations already reach.
func (b *builder) unwanted(
	ctx context.Context, rm *RelationMapping, stored []*entity.Relation, want map[string]bool, t *relTarget,
) (map[string]bool, error) {
	have := map[string]bool{}
	for _, r := range stored {
		other := r.To
		if !rm.out() {
			other = r.From
		}
		have[other] = true
		e := t.candidate(other)
		if want[other] || e == nil {
			continue
		}
		_, ok, err := b.usable(ctx, e, t.ident)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		change := RelationChange{Type: r.Type, From: r.From, To: r.To}
		if rm.Prune {
			b.plan.removes = append(b.plan.removes, r.Identity())
			b.plan.RelationRemoves = append(b.plan.RelationRemoves, change)
		} else {
			b.plan.Stale = append(b.plan.Stale, change)
		}
	}
	return have, nil
}

// addRelations plans the relations in adds that are not stored yet.
func (b *builder) addRelations(adds []*relOp, have map[string]bool, out bool) {
	for _, op := range adds {
		other := op.to
		if !out {
			other = op.from
		}
		if other.new == nil && have[other.id] {
			continue
		}
		label := op.typ + "\x00" + op.from.label() + "\x00" + op.to.label()
		if b.addSeen[label] {
			continue
		}
		b.addSeen[label] = true
		b.plan.adds = append(b.plan.adds, op)
		b.plan.RelationAdds = append(b.plan.RelationAdds,
			RelationChange{Type: op.typ, From: op.from.label(), To: op.to.label()})
	}
}

// missing reports stored rows in scope that the source no longer holds.
func (b *builder) missing(ctx context.Context, st *mappingState) error {
	ident := st.ident()
	for _, key := range sortedKeys(st.inScope) {
		if st.seen[key] {
			continue
		}
		for _, e := range st.inScope[key] {
			g, ok, err := b.usable(ctx, e, ident)
			if err != nil {
				return err
			}
			if ok {
				b.plan.Missing = append(b.plan.Missing,
					MissingEntity{Mapping: st.em.name, Type: g.Type, ID: g.ID, Key: key})
			}
		}
	}
	return nil
}

// sameValue compares a stored value with an imported one, through the same
// normal form on both sides: an external ref by its id, numbers and dates
// by their text.
func sameValue(pd metamodel.PropertyDef, old, imported any) bool {
	if pd.Type == metamodel.PropertyTypeExternalRef {
		a, _ := metamodel.ExternalRefID(old)
		b, _ := metamodel.ExternalRefID(imported)
		return a == b
	}
	return normalize(old) == normalize(imported)
}

// normalize renders a property value as comparable text.
func normalize(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float64:
		if t == math.Trunc(t) && math.Abs(t) < 1e15 {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case time.Time:
		if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 && t.Nanosecond() == 0 {
			return t.Format(time.DateOnly)
		}
		return t.Format(time.RFC3339Nano)
	case []any:
		parts := make([]string, len(t))
		for i, x := range t {
			parts[i] = normalize(x)
		}
		return strings.Join(parts, "\x1f")
	case []string:
		return strings.Join(t, "\x1f")
	case map[string]any:
		if id, ok := t["id"]; ok {
			return normalize(id)
		}
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(data)
}

func trimBody(s string) string { return strings.TrimRight(s, " \t\r\n") }
