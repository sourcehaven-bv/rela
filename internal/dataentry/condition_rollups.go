package dataentry

import (
	"context"
	"log/slog"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/relresolve"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// conditionAttributes is the optional capability a [ViewConditionMatcher]
// offers to say which entity fields its condition reads.
type conditionAttributes interface {
	EntityType() string
	EntityAttributes() []string
}

// withConditionRollups wraps fn so a condition that reads roll-ups of its
// type is judged on rows carrying the reader's roll-up values. Only a list
// `condition:` compiles with roll-ups (conditionlint), so only those are
// wrapped. src computes the values per request, so they are per principal.
func withConditionRollups(fn ViewConditionFunc, src rollupSource) ViewConditionFunc {
	return func(cfg *dataentryconfig.Config, meta *metamodel.Metamodel) (ViewConditionLookup, []string) {
		lookup, problems := fn(cfg, meta)
		if lookup == nil {
			return nil, problems
		}
		return func(kind, id string) (ViewConditionMatcher, bool) {
			m, ok := lookup(kind, id)
			if !ok {
				return nil, false
			}
			if want := conditionRollups(meta, m); len(want) > 0 {
				return rollupConditionMatcher{inner: m, want: want, src: src}, true
			}
			return m, true
		}, problems
	}
}

// conditionRollups returns the roll-ups of its type that m reads, or nil
// when m does not say what it reads.
func conditionRollups(meta *metamodel.Metamodel, m ViewConditionMatcher) map[string]bool {
	attrs, ok := m.(conditionAttributes)
	if !ok || meta == nil {
		return nil
	}
	names := meta.RollupNames(attrs.EntityType())
	var want map[string]bool
	for _, attr := range attrs.EntityAttributes() {
		if slices.Contains(names, attr) {
			if want == nil {
				want = map[string]bool{}
			}
			want[attr] = true
		}
	}
	return want
}

// rollupConditionMatcher judges a condition that reads roll-ups. Before it
// evaluates, it computes those roll-ups for the rows it is handed, which are
// the redacted candidates, gated for the reader inside the roll-up query
// (gate before fold). Its store calls grow with the candidates over
// [rollupChunk] (TestQueryBudget_ListConditionRollups).
//
// Its store lowering ([rollupConditionMatcher.Lower]) answers each exists
// roll-up compared with true or false by a per-candidate walk inside the
// page scan, when queryplan admits it (design gap 3.5, step 2); min and
// max, defer-date conditions (TKT-4Z2BIRG) and nested roll-ups keep the
// Go path.
type rollupConditionMatcher struct {
	inner ViewConditionMatcher
	want  map[string]bool
	src   rollupSource
}

// EntityType is the type the inner condition was compiled against.
func (m rollupConditionMatcher) EntityType() string {
	attrs, ok := m.inner.(conditionAttributes)
	if !ok {
		return ""
	}
	return attrs.EntityType()
}

// EntityAttributes are the entity fields the inner condition reads.
func (m rollupConditionMatcher) EntityAttributes() []string {
	attrs, ok := m.inner.(conditionAttributes)
	if !ok {
		return nil
	}
	return attrs.EntityAttributes()
}

// rollupLowerer is the optional capability of a compiled condition to lower
// its roll-up comparisons; appbuild's view condition matcher has it.
type rollupLowerer interface {
	LowerRollups(
		ctx context.Context, gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
		facts queryplan.RollupFacts, bind visibility.RollupLeafBinder,
	) (frag store.GraphQuery, empty, ok bool)
}

// Lower rewrites the condition for the principal gate authorizes, each
// roll-up comparison queryplan admits bound to this reader's walk in the
// request's world. ok=false keeps the Go path: the inner condition cannot
// lower its roll-ups, or the reader's walk has no store form.
func (m rollupConditionMatcher) Lower(
	ctx context.Context, gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
) (frag store.GraphQuery, empty, ok bool) {
	l, lowers := m.inner.(rollupLowerer)
	if !lowers || m.src.schema == nil || m.src.schema() == nil {
		return frag, false, false
	}
	set := m.src.schema().rollups
	reads := visibility.ReadQueryFunc(func(ctx context.Context, t string) (acl.ReadQueryResult, error) {
		return readGateFromContext(ctx).ReadQuery(ctx, t), nil
	})
	r, err := visibility.NewRollups(set, m.src.st, reads, m.src.fields())
	if err != nil {
		return frag, false, false
	}
	world := worldScopeFrom(ctx)
	bind := func(ctx context.Context, entityType, property string) (store.RollupQuery, bool, error) {
		return r.LeafQuery(ctx, entityType, property, world)
	}
	return l.LowerRollups(ctx, gate, set.LeafFacts(r.OrderFieldGated), bind)
}

// MatchPage computes the roll-ups the condition reads for rows, one batch
// per entity type, and judges the copies that carry them. A row on which a
// roll-up the condition reads is absent, because it could not be computed
// (over its row cap, a failed store call, a withheld value), is no match:
// the condition fails closed whatever it does with the value, `not`
// included.
func (m rollupConditionMatcher) MatchPage(
	ctx context.Context, rows []*entityPkg.Entity, gate relresolve.Gate, match relresolve.Match,
) ([]bool, error) {
	copies, absent := m.withRollups(ctx, rows)
	out, err := m.inner.MatchPage(ctx, copies, gate, match)
	if err != nil {
		return out, err
	}
	for i := range out {
		if i < len(absent) && absent[i] {
			out[i] = false
		}
	}
	return out, nil
}

// withRollups returns copies of rows carrying the wanted roll-ups, in order,
// and per row whether one of them is absent.
func (m rollupConditionMatcher) withRollups(
	ctx context.Context, rows []*entityPkg.Entity,
) (out []*entityPkg.Entity, absent []bool) {
	byType := map[string][]int{}
	var order []string
	for i, e := range rows {
		if _, seen := byType[e.Type]; !seen {
			order = append(order, e.Type)
		}
		byType[e.Type] = append(byType[e.Type], i)
	}
	out = make([]*entityPkg.Entity, len(rows))
	absent = make([]bool, len(rows))
	for _, typ := range order {
		idx := byType[typ]
		group := make([]*entityPkg.Entity, len(idx))
		for j, i := range idx {
			group[j] = rows[i]
		}
		copies, gone := m.src.conditionCopies(ctx, typ, group, m.want)
		for j, c := range copies {
			out[idx[j]], absent[idx[j]] = c, gone[j]
		}
	}
	return out, absent
}

// conditionCopies is [rollupSource.copiesWithRollups] for a condition: it
// also reports, per entity, whether a roll-up in want is absent because it
// could not be computed ([visibility.Rollups.ValuesOrAbsent]). When the
// roll-up reader cannot be built, every entity has one absent.
func (s rollupSource) conditionCopies(
	ctx context.Context, typeName string, entities []*entityPkg.Entity, want map[string]bool,
) (copies []*entityPkg.Entity, gone []bool) {
	copies = make([]*entityPkg.Entity, len(entities))
	refs := make([]entityPkg.Ref, len(entities))
	for i, e := range entities {
		cp := *e
		cp.Properties = maps.Clone(e.Properties)
		if cp.Properties == nil {
			cp.Properties = map[string]any{}
		}
		copies[i], refs[i] = &cp, e.Ref()
	}
	gone = make([]bool, len(entities))
	if s.schema == nil || s.schema() == nil {
		return copies, gone
	}
	sch := s.schema()
	values, absent := s.conditionValues(ctx, sch, typeName, refs, want)
	names := sch.Meta.RollupNames(typeName)
	for i, cp := range copies {
		visibility.MergeRollups(cp.Properties, names, values[cp.ID])
		for p := range want {
			gone[i] = gone[i] || absent[cp.ID][p]
		}
	}
	return copies, gone
}

// conditionValues computes the roll-ups in want of refs, all of typeName,
// one call per [rollupChunk] refs, with the absent ones keyed by id.
func (s rollupSource) conditionValues(
	ctx context.Context, sch *Schema, typeName string, refs []entityPkg.Ref, want map[string]bool,
) (values map[string]map[string]any, absent map[string]map[string]bool) {
	values, absent = map[string]map[string]any{}, map[string]map[string]bool{}
	if len(sch.rollups.For(typeName)) == 0 {
		return values, absent
	}
	gate := visibility.ReadQueryFunc(func(ctx context.Context, t string) (acl.ReadQueryResult, error) {
		return readGateFromContext(ctx).ReadQuery(ctx, t), nil
	})
	r, err := visibility.NewRollups(sch.rollups, s.st, gate, s.fields())
	if err != nil {
		slog.WarnContext(ctx, "condition roll-ups not computed", "type", typeName, "error", err)
		for _, ref := range refs {
			absent[ref.ID] = maps.Clone(want)
		}
		return values, absent
	}
	r = s.withRecords(r)
	world := worldScopeFrom(ctx)
	for chunk := range slices.Chunk(refs, rollupChunk) {
		v, a := r.ValuesOrAbsent(ctx, typeName, chunk, world, func(p string) bool { return want[p] })
		maps.Copy(values, v)
		maps.Copy(absent, a)
	}
	return values, absent
}
