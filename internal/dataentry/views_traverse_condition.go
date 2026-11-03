package dataentry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// viewKindTraverse is a view traverse step's `condition:`, keyed by
// dataentryconfig.TraverseConditionID.
const viewKindTraverse = "traverse"

// errTraverseConditionUnavailable fails a view whose traverse condition has
// no compiled program. Skipping the condition would widen a step whose job is
// to narrow (BUG-WHEREWIDE); validation refuses the config that gets here.
var errTraverseConditionUnavailable = errors.New("view traverse condition is not compiled")

// traverseFilter applies the `where:` and `condition:` of a view's traverse
// steps. One is built per view execution, so the configured conditions are
// compiled once per request rather than once per step and fixpoint pass.
//
// A type of its own rather than viewsHandler methods: the handler is at its
// plimsoll method load line.
type traverseFilter struct {
	schema   *Schema
	lookup   ViewConditionLookup // nil when no step declares a condition
	redactor visibility.FieldRedactor
	store    store.GraphQueryer
}

// newTraverseFilter resolves the compiled traverse conditions for view. A
// view without conditions compiles nothing. A view with one fails when no
// compiler is wired or the conditions do not compile.
func newTraverseFilter(h *viewsHandler, view ViewConfig) (*traverseFilter, error) {
	f := &traverseFilter{schema: h.schema(), redactor: h.redactor(), store: h.store}
	if !hasTraverseCondition(view) {
		return f, nil
	}
	var fn ViewConditionFunc
	if h.viewConditions != nil {
		fn = h.viewConditions()
	}
	if fn == nil || f.schema == nil {
		return nil, errTraverseConditionUnavailable
	}
	lookup, problems := fn(f.schema.Cfg, f.schema.Meta)
	if len(problems) > 0 || lookup == nil {
		return nil, fmt.Errorf("%w: %s", errTraverseConditionUnavailable, strings.Join(problems, "; "))
	}
	f.lookup = lookup
	return f, nil
}

func hasTraverseCondition(view ViewConfig) bool {
	for _, t := range view.Traverse {
		if t.Condition != "" {
			return true
		}
	}
	return false
}

// narrow applies a step's `where:` and then its `condition:`.
func (f *traverseFilter) narrow(
	ctx context.Context, rule ViewTraverse, found []*entity.Entity,
) ([]*entity.Entity, error) {
	if rule.Where != "" {
		filtered, err := filterEntities(f.schema, found, rule.Where)
		if err != nil {
			return nil, err // refused at load (BUG-WHEREWIDE); reaching here is a config bug
		}
		found = filtered
	}
	return f.applyCondition(ctx, rule, found)
}

// applyCondition keeps the entities of found that satisfy the step's
// `condition:`, in order.
//
// Each entity type has its own program, so found is judged one type at a
// time. An entity whose type the step cannot collect by the schema (an edge
// stored before a relation's types changed) has no program and is dropped:
// narrowing is the safe direction. A type the step CAN collect without a
// program is an error, never a silent drop: the config changed between the
// view's snapshot and this one, or the two sides disagree on the key.
//
// The condition reads the REDACTED entity, as a list condition does (see
// [applyViewCondition]), so it cannot decide membership on a field value the
// reader may not see. `where:` keeps reading raw values, as documented on
// [viewsHandler.executeViewBodies].
func (f *traverseFilter) applyCondition(
	ctx context.Context, rule ViewTraverse, found []*entity.Entity,
) ([]*entity.Entity, error) {
	if rule.Condition == "" || len(found) == 0 {
		return found, nil
	}
	if f.lookup == nil {
		return nil, errTraverseConditionUnavailable
	}
	types := dataentryconfig.TraverseConditionTypes(rule, f.schema.Meta)
	byType := map[string][]*entity.Entity{}
	var order []string
	for _, e := range found {
		if _, ok := byType[e.Type]; !ok {
			order = append(order, e.Type)
		}
		byType[e.Type] = append(byType[e.Type], e)
	}
	redact := func(ctx context.Context, e *entity.Entity) *entity.Entity {
		return visibility.Redact(ctx, f.redactor, e)
	}
	kept := make(map[*entity.Entity]bool, len(found))
	for _, typ := range order {
		id := dataentryconfig.TraverseConditionID(typ, types, rule.Condition)
		m := viewConditionFor(f.lookup, viewKindTraverse, id)
		if m == nil {
			if slices.Contains(types, typ) {
				return nil, fmt.Errorf("traverse step %q: %w for type %q", rule.CollectAs, errTraverseConditionUnavailable, typ)
			}
			continue
		}
		rows, err := applyViewCondition(ctx, byType[typ], m, redact, f.store)
		if err != nil {
			return nil, fmt.Errorf("traverse step %q condition: %w", rule.CollectAs, err)
		}
		for _, e := range rows {
			kept[e] = true
		}
	}
	out := make([]*entity.Entity, 0, len(kept))
	for _, e := range found {
		if kept[e] {
			out = append(out, e)
		}
	}
	return out, nil
}
