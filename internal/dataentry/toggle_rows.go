package dataentry

import (
	"context"
	"fmt"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// Toggle columns (TKT-FJG779Q).
//
// A list column with `widget: toggle` is a checkbox. Each row of a read that
// names the list carries `_toggles`: per toggle property, the value a click
// writes, present only when the caller may write it now. The flip rule lives
// here, so the SPA only sends what it was given; the PATCH checks it again.

// transitionPrimer is the OPTIONAL capability of a [FieldVerdictResolver]
// that binds the transition `when:` traversals of a page of rows at once.
// Only the policy-backed resolver has it.
type transitionPrimer interface {
	PrimeTransitionTraversals(ctx context.Context, entityType string, ids []string) context.Context
}

// toggleSources is what serveRowToggles reads: the schema, the verdict
// resolver and the affordance service of one request.
type toggleSources struct {
	schema *Schema
	// listType is the entity type of the list; a row of another type gets
	// no toggles, even when it has a property of the same name.
	listType string
	res      FieldVerdictResolver
	svc      affordanceService
}

// listToggleColumns returns the list a read names and its toggle columns.
// The columns are nil when the read names no list or the list has none.
func listToggleColumns(s *Schema, query map[string][]string) (listType string, cols []dataentryconfig.ListColumn) {
	listID := queryGet(query, listIDParam)
	if listID == "" || s == nil || s.Cfg == nil {
		return "", nil
	}
	list, ok := s.Cfg.Lists[listID]
	if !ok {
		return "", nil
	}
	var out []dataentryconfig.ListColumn
	for _, c := range list.Columns {
		if c.IsToggle() && c.Property != "" {
			out = append(out, c)
		}
	}
	return list.EntityType, out
}

// serveRowToggles sets `_toggles` on each row in data, which is parallel to
// rows. The page is primed once (field verdicts, flow runs, transition
// traversals), so the extra adds no store read per row.
func serveRowToggles(
	ctx context.Context, src toggleSources, cols []dataentryconfig.ListColumn,
	rows []*entityPkg.Entity, data []v1.Entity,
) {
	if len(cols) == 0 || len(rows) == 0 || src.schema == nil || src.schema.Meta == nil || src.res == nil {
		return
	}
	meta := src.schema.Meta
	tr, _ := src.res.(TransitionResolver)
	ctx = primeVerdicts(ctx, src.res, rows)
	ctx = primeFlowRuns(ctx, src.res, rows)
	ctx = primeToggleTransitions(ctx, src.res, tr, src.listType, cols, rows)
	for i, e := range rows {
		if i >= len(data) || e == nil {
			continue
		}
		data[i].Toggles = rowToggles(ctx, src, meta, tr, cols, e)
	}
}

// primeToggleTransitions binds the transition traversals of the page's rows
// of listType once, when a toggle column's property is a state machine. A row
// of a variant type is answered on its own, which is correct, only slower.
func primeToggleTransitions(
	ctx context.Context, res FieldVerdictResolver, tr TransitionResolver, listType string,
	cols []dataentryconfig.ListColumn, rows []*entityPkg.Entity,
) context.Context {
	p, ok := res.(transitionPrimer)
	if !ok || tr == nil {
		return ctx
	}
	entries := tr.EntryValues(listType)
	machine := false
	for _, c := range cols {
		if _, isMachine := entries[c.Property]; isMachine {
			machine = true
		}
	}
	if !machine {
		return ctx
	}
	ids := make([]string, 0, len(rows))
	for _, e := range rows {
		if e != nil && e.Face == "" && e.Type == listType {
			ids = append(ids, e.ID) // a named face fails a traversing when: closed anyway
		}
	}
	return p.PrimeTransitionTraversals(ctx, listType, ids)
}

// rowToggles computes the `_toggles` of one row; nil when it has none.
func rowToggles(
	ctx context.Context, src toggleSources, meta *metamodel.Metamodel, tr TransitionResolver,
	cols []dataentryconfig.ListColumn, e *entityPkg.Entity,
) map[string]any {
	def, ok := meta.GetEntityDef(e.Type)
	if !ok || (e.Type != src.listType && def.Extends != src.listType) {
		return nil
	}
	var entries map[string]string
	if tr != nil {
		entries = tr.EntryValues(e.Type)
	}
	fv := src.svc.fieldVerdicts(ctx, e)
	out := map[string]any{}
	for _, c := range cols {
		prop, declared := def.Properties[c.Property]
		if !declared || !fv.IsVisible(c.Property) || !fv.IsWritable(c.Property) {
			continue
		}
		current := toggleCurrent(e, c, entries[c.Property])
		target, flips := toggleTarget(c, current)
		if !flips || !fv.IsOptionAllowed(c.Property, target) {
			continue
		}
		_, isMachine := entries[c.Property]
		if isMachine && !transitionAllowed(ctx, tr, e, c.Property, current, target) {
			continue
		}
		if prop.Type == metamodel.PropertyTypeBoolean {
			out[c.Property] = target == "true"
		} else {
			out[c.Property] = target
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// toggleCurrent is the row's value of the toggle property as the write path
// sees it: an unset machine property reads as its entry value, any other
// unset property as Off.
func toggleCurrent(e *entityPkg.Entity, c dataentryconfig.ListColumn, entry string) string {
	v, ok := e.Properties[c.Property]
	if !ok || v == nil || v == "" {
		if entry != "" {
			return entry
		}
		return c.Off
	}
	return fmt.Sprint(v)
}

// toggleTarget is the value a click writes from current, and whether a
// click writes at all.
func toggleTarget(c dataentryconfig.ListColumn, current string) (target string, flips bool) {
	switch {
	case current == c.On:
		return c.Off, true
	case current == c.Off:
		return c.On, true
	case c.Other == dataentryconfig.ToggleOtherOn:
		return c.Off, true
	default:
		return "", false
	}
}

// transitionAllowed reports whether the caller may move prop of e from
// current to target now. An unset property is judged on a clone that holds
// current (its entry value), because the machine has no state to leave
// from otherwise.
func transitionAllowed(
	ctx context.Context, tr TransitionResolver, e *entityPkg.Entity, prop, current, target string,
) bool {
	if tr == nil {
		return false
	}
	subject := e
	if e.GetString(prop) != current {
		subject = e.Clone()
		subject.SetString(prop, current)
	}
	for _, v := range tr.TransitionVerdicts(ctx, subject)[prop] {
		if v.To == target {
			return v.Allowed
		}
	}
	return false
}
