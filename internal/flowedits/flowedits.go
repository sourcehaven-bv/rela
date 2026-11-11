// Package flowedits answers a flow's `outside_edits:` (TKT-XI7IDBS): which
// properties of a subject stay writable outside the flow while a run of the
// flow holds the subject.
//
// A [Ceiling] is the [affordances.FlowEditCeiling] the wiring site gives the
// field verdicts. It reads the flow run store, which holds exactly the runs
// that have not ended (a run is deleted when it ends), so it does not
// depend on the `_flow-run` header, which can lag the run. The rules come
// from the current flows.yaml, by flow name, like acl.yaml.
//
// A write the flow makes itself is not outside the run: [WithinRun] marks
// it, and the run it names does not narrow it. Every other run on the
// subject still does.
package flowedits

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunLister is the part of [flowrun.Store] a [Ceiling] reads.
type RunLister interface {
	List(ctx context.Context, q flowrun.Query) ([]flowrun.Run, error)
}

// run is what the ceiling needs of a stored run.
type run struct {
	id   flowrun.RunID
	flow string
	// state is the stored instance's state. Empty: the instance has not
	// left its start, so the run is in the flow's initial state.
	state string
}

// Ceiling narrows the writable properties of the subjects that runs hold.
// Safe for concurrent use.
type Ceiling struct {
	runs RunLister
	// flows holds, by name, the flows that declare `outside_edits:`.
	flows map[string]*flowcfg.Flow
	// governed holds the subject types of those flows.
	governed map[string]bool
	// properties holds, per subject type, the properties a rule can
	// restrict: declared, not derived and not reserved.
	properties map[string][]string
}

// New returns the ceiling of flows over runs, or nil when no flow declares
// `outside_edits:`.
//
// Nil: flows is accepted (no flows: nil ceiling); meta and runs are
// rejected when some flow declares `outside_edits:`.
func New(flows *flowcfg.Compiled, meta *metamodel.Metamodel, runs RunLister) (*Ceiling, error) {
	if flows == nil {
		return nil, nil //nolint:nilnil // no flows: no ceiling
	}
	c := &Ceiling{runs: runs, flows: map[string]*flowcfg.Flow{}, governed: map[string]bool{},
		properties: map[string][]string{}}
	for _, f := range flows.Flows() {
		if f.OutsideEdits() == nil {
			continue
		}
		c.flows[f.Name()] = f
		c.governed[f.Subject()] = true
	}
	if len(c.flows) == 0 {
		return nil, nil //nolint:nilnil // no outside_edits: no ceiling
	}
	if meta == nil || runs == nil {
		return nil, errors.New("flowedits: New requires a metamodel and a run store")
	}
	for typ := range c.governed {
		def, ok := meta.GetEntityDef(typ)
		if !ok {
			return nil, fmt.Errorf("flowedits: schema.yaml does not declare %q", typ)
		}
		var props []string
		for name, pd := range def.Properties {
			if pd.Computed != "" || pd.Rollup != nil || metamodel.IsReservedName(name) {
				continue
			}
			props = append(props, name)
		}
		slices.Sort(props)
		c.properties[typ] = props
	}
	return c, nil
}

// Governs implements [affordances.FlowEditCeiling].
//
// Nil: accepted; a nil ceiling governs nothing.
func (c *Ceiling) Governs(entityType string) bool {
	return c != nil && c.governed[entityType]
}

// Permissions implements [affordances.FlowEditCeiling]: the permissions
// the rules of every flow on entityType ask for, sorted.
//
// Nil: accepted; a nil ceiling asks for none.
func (c *Ceiling) Permissions(entityType string) []string {
	if c == nil {
		return nil
	}
	var out []string
	for _, f := range c.flows {
		if f.Subject() == entityType {
			out = append(out, f.OutsideEdits().Permissions()...)
		}
	}
	return slices.Compact(slices.Sorted(slices.Values(out)))
}

// memoKey is the ctx key of the runs primed for a page.
type memoKey struct{}

// memo holds the runs on a set of subject ids, as one lookup found them.
// It lives on the ctx of one read and is never shared beyond it.
type memo struct {
	mu   sync.Mutex
	runs map[string][]run
}

// PrimeRuns implements [affordances.FlowEditCeiling].
func (c *Ceiling) PrimeRuns(ctx context.Context, ids []string) (context.Context, error) {
	found, err := c.activeRuns(ctx, ids)
	if err != nil {
		return ctx, err
	}
	m, ok := ctx.Value(memoKey{}).(*memo)
	if !ok {
		m = &memo{runs: map[string][]run{}}
		ctx = context.WithValue(ctx, memoKey{}, m)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range ids {
		m.runs[id] = found[id]
	}
	return ctx, nil
}

// Withheld implements [affordances.FlowEditCeiling]. A property is withheld
// when, for some run on e other than the one [WithinRun] names, the rule of
// the run's flow and state is deny, or asks for a permission holds does not
// grant. The value names the flow and state of the first such run, by run
// id: `outside_edits:flow=F state=S`.
func (c *Ceiling) Withheld(
	ctx context.Context, e *entity.Entity, holds func(permission string) bool,
) (map[string]string, error) {
	if e == nil || !c.Governs(e.Type) {
		return nil, nil //nolint:nilnil // nothing withheld: the interface's empty answer
	}
	runs, err := c.runsOn(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	within, _ := ctx.Value(withinKey{}).(flowrun.RunID)
	var out map[string]string
	for _, r := range runs {
		f, ok := c.flows[r.flow]
		if !ok || f.Subject() != e.Type || (within != "" && r.id == within) {
			continue
		}
		rules := f.OutsideEdits()
		state := r.state
		if state == "" {
			state = f.Initial()
		}
		for _, prop := range c.properties[e.Type] {
			if _, done := out[prop]; done {
				continue
			}
			switch rule := rules.Rule(state, prop); rule.Kind {
			case flowcfg.EditDeny:
			case flowcfg.EditPermission:
				if holds(rule.Permission) {
					continue
				}
			default:
				continue
			}
			if out == nil {
				out = map[string]string{}
			}
			out[prop] = "outside_edits:flow=" + r.flow + " state=" + state
		}
	}
	return out, nil
}

// runsOn returns the runs on id: primed, or one lookup. Inside a store
// transaction it always looks up: the check the manager repeats there must
// see the runs as they are at the write, not as a page primed them.
func (c *Ceiling) runsOn(ctx context.Context, id string) ([]run, error) {
	if m, ok := ctx.Value(memoKey{}).(*memo); ok && !store.InTx(ctx) {
		m.mu.Lock()
		runs, primed := m.runs[id]
		m.mu.Unlock()
		if primed {
			return runs, nil
		}
	}
	found, err := c.activeRuns(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return found[id], nil
}

// activeRuns returns the stored runs of a governing flow on ids, by subject
// id, ordered by run id. It asks the run store for at most
// [flowrun.MaxPage] ids at a time and pages each batch, so any number of
// ids and runs is read.
func (c *Ceiling) activeRuns(ctx context.Context, ids []string) (map[string][]run, error) {
	unique := slices.Compact(slices.Sorted(slices.Values(ids)))
	out := map[string][]run{}
	for batch := range slices.Chunk(unique, flowrun.MaxPage) {
		var after flowrun.RunID
		for {
			page, err := c.runs.List(ctx, flowrun.Query{SubjectIDs: batch, After: after, Limit: flowrun.MaxPage})
			if err != nil {
				return nil, fmt.Errorf("flowedits: list runs: %w", err)
			}
			for _, r := range page {
				if _, governs := c.flows[r.Flow]; governs {
					out[r.Subject.ID] = append(out[r.Subject.ID], run{id: r.ID, flow: r.Flow, state: r.Instance.State})
				}
			}
			if len(page) < flowrun.MaxPage {
				break
			}
			after = page[len(page)-1].ID
		}
	}
	return out, nil
}
