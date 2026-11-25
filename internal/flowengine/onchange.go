package flowengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
)

// OnChange acts on the ordinary writes to a subject: it starts the runs of
// the flows whose `on: predicate` start the write makes the subject come
// to satisfy, and fires the `change` event on the subject's open runs. It
// is a type of its own so that [Engine] stays within its method cap.
type OnChange struct{ e *Engine }

// NewOnChange returns the write events of e.
//
// Nil: e rejected; the caller passes the engine it built.
func NewOnChange(e *Engine) *OnChange { return &OnChange{e: e} }

// Write is one update of a subject, as [OnChange.Changed] sees it.
type Write struct {
	// Runs is the run store, joined to the write's transaction where the
	// backend has one.
	Runs flowrun.Store
	// Old is the row before the write and New the row as written.
	// Nil: rejected; an update always has both rows.
	Old, New *entity.Entity
	// By is the principal who made the write.
	By string
	// Holds reports whether By holds a role on the subject.
	// Nil: rejected; the caller passes one that holds nothing to fail
	// closed.
	Holds func(role string) bool
	// Depth is the depth of the runs the write starts. Starts is false
	// when the write may start no run, because a chain of starts reached
	// its bound.
	Depth  int
	Starts bool
	// Machine marks a write a run made itself (an output or a job): it
	// fires no `change` event on any run of the subject, so two runs on
	// one subject cannot move each other back and forth. It may still
	// start a run on a predicate, within the depth bound.
	Machine bool
	// Skip is the run whose step the write completes, which takes its
	// event itself; empty for none.
	Skip flow.RunID
}

// Changes is what [OnChange.Changed] did: the runs it started, and the
// runs to settle once the write has committed.
type Changes struct {
	Started []flow.RunID
	Settle  []flow.RunID
}

// Changed acts on w inside the write's store transaction and stores every
// start and move on w.Runs, so each commits or rolls back with the write.
// A move is stored pending; a token still settling keeps the event on its
// mark. The caller settles [Changes.Settle] after the commit, or a round of
// [Engine.Due] does. An ordinary write takes a `change` branch only when
// its `when:` reads a changed property, its roles admit w.By and it holds
// on w.New; a step claimed by another principal is left to them. Guards
// read w.New, never the store, which may still serve the old row.
func (o *OnChange) Changed(ctx context.Context, w Write) (Changes, error) {
	changed := ChangedProps(w.Old, w.New)
	if len(changed) == 0 {
		return Changes{}, nil
	}
	te := o.inTx(w)
	var open []flowrun.Run
	if !w.Machine {
		var err error
		if open, err = subjectRuns(ctx, w.Runs, w.New); err != nil {
			return Changes{}, err
		}
	}
	var out Changes
	if w.Starts {
		started, err := te.startPredicates(ctx, w, changed)
		if err != nil {
			return Changes{}, err
		}
		out.Started = started
	}
	for _, run := range open {
		if run.ID == w.Skip {
			continue
		}
		settle, err := te.changeRunRetried(ctx, w, changed, run)
		if err != nil {
			return Changes{}, err
		}
		if settle {
			out.Settle = append(out.Settle, run.ID)
		}
	}
	out.Settle = append(out.Settle, out.Started...)
	return out, nil
}

// inTx returns the engine that acts inside the write's transaction: it
// stores runs on w.Runs, and refuses to read a subject from the store.
func (o *OnChange) inTx(w Write) *OnChange {
	d := o.e.d
	d.Runs, d.Subjects = w.Runs, noSubjects{}
	g := o.e.g
	g.subjects = d.Subjects
	return &OnChange{e: &Engine{d: d, g: g}}
}

// errSubjectInTx is a subject read inside a write's transaction.
var errSubjectInTx = errors.New("flowengine: a write event reads its subject from the write, not the store")

// noSubjects refuses every read: see OnChange.inTx.
type noSubjects struct{}

func (noSubjects) Subject(context.Context, entity.Ref) (*entity.Entity, error) {
	return nil, errSubjectInTx
}

func (noSubjects) SubjectAs(context.Context, string, entity.Ref) (map[string]any, error) {
	return nil, errSubjectInTx
}

// ChangedProps returns the names of the properties whose value differs
// between old and e. Values that encode to the same JSON are equal, so a
// number read back as another numeric type is not a change.
//
// Nil: old and e rejected; an update always has both rows.
func ChangedProps(old, e *entity.Entity) map[string]bool {
	out := map[string]bool{}
	for k, v := range e.Properties {
		if ov, ok := old.Properties[k]; !ok || !sameValue(ov, v) {
			out[k] = true
		}
	}
	for k := range old.Properties {
		if _, ok := e.Properties[k]; !ok {
			out[k] = true
		}
	}
	return out
}

// sameValue reports whether a and b are equal, or encode to the same JSON.
func sameValue(a, b any) bool {
	if reflect.DeepEqual(a, b) {
		return true
	}
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && bytes.Equal(ja, jb)
}

// reads reports whether g reads one of changed.
func reads(g *flowcfg.Guard, changed map[string]bool) bool {
	return g != nil && slices.ContainsFunc(g.Fields(), func(f string) bool { return changed[f] })
}

// subjectRuns returns the open runs on subj's face.
func subjectRuns(ctx context.Context, runs flowrun.Store, subj *entity.Entity) ([]flowrun.Run, error) {
	var out []flowrun.Run
	q := flowrun.Query{SubjectID: subj.ID, Limit: flowrun.MaxPage}
	for {
		page, err := runs.List(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("flowengine: list the runs on %s: %w", subj.ID, err)
		}
		for _, r := range page {
			if r.Subject.Face == subj.Face {
				out = append(out, r)
			}
		}
		if len(page) < q.Limit {
			return out, nil
		}
		q.After = page[len(page)-1].ID
	}
}

// MayStart reports whether a write to an entity of type typ that changed
// the properties in changed can start a run on a predicate: some flow on
// typ has an `on: predicate` start whose `when:` reads one of them.
func (o *OnChange) MayStart(typ string, changed map[string]bool) bool {
	for _, f := range o.e.d.Flows.Flows() {
		if f.Subject() != typ {
			continue
		}
		if slices.ContainsFunc(startsOn(f, flowcfg.StartPredicate), func(s flowcfg.Start) bool {
			return reads(s.When, changed)
		}) {

			return true
		}
	}
	return false
}

// startPredicates starts a run of every flow on w.New's type whose
// `on: predicate` start holds on w.New and held on w.Old for none of its
// predicate starts. A start whose `when:` reads no changed property
// cannot have come to hold, so it is not evaluated.
func (o *OnChange) startPredicates(ctx context.Context, w Write, changed map[string]bool) ([]flow.RunID, error) {
	e := o.e
	var ids []flow.RunID
	for _, f := range e.d.Flows.Flows() {
		if f.Subject() != w.New.Type {
			continue
		}
		starts := startsOn(f, flowcfg.StartPredicate)
		if !slices.ContainsFunc(starts, func(s flowcfg.Start) bool { return reads(s.When, changed) }) {
			continue
		}
		if anyStartHolds(ctx, e, f, starts, w.Old) || !anyStartHolds(ctx, e, f, starts, w.New) {
			continue
		}
		fctx := audit.WithFlowStep(ctx, audit.FlowStep{By: f.RunAs()})
		run, err := begin(fctx, e, w.Runs, f, w.New, w.Depth)
		if errors.Is(err, ErrSubjectBusy) {
			slog.WarnContext(ctx, "flowengine: no run started on a predicate: the subject already carries one",
				"flow", f.Name(), "subject", w.New.ID)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("flowengine: start flow %s on %s: %w", f.Name(), w.New.ID, err)
		}
		ids = append(ids, run.ID)
	}
	return ids, nil
}

// maxEventTries bounds how often a write event is decided again on a run
// that another writer changed under it.
const maxEventTries = 3

// errEventLost is a write event that lost every try to other writers of
// its run. It fails the write, which the caller may retry.
var errEventLost = errors.New("flowengine: a run changed under every try of a write event")

// changeRunRetried is changeRun, decided again on the run as stored when
// another writer, such as the flow worker, changed it first. The tokens
// the event already moved are not moved twice. A run that ended meanwhile
// takes no event.
func (o *OnChange) changeRunRetried(ctx context.Context, w Write, changed map[string]bool,
	run flowrun.Run) (bool, error) {
	id, acted := run.ID, map[string]bool{}
	for range maxEventTries {
		settle, lost, err := o.changeRun(ctx, w, changed, run, acted)
		if err != nil || !lost {
			return settle, err
		}
		run, err = w.Runs.Get(ctx, id)
		if errors.Is(err, flowrun.ErrNotFound) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("flowengine: read run %s again: %w", id, err)
		}
	}
	return false, fmt.Errorf("%w: %s", errEventLost, id)
}

// changeRun fires the `change` event of w on each live token of run that
// is not in acted, and adds the tokens it moved to acted. It reports
// whether run needs settling after the commit (a token moved or kept the
// event, or a join waits) and whether another writer changed run first.
func (o *OnChange) changeRun(ctx context.Context, w Write, changed map[string]bool,
	run flowrun.Run, acted map[string]bool) (settle, lost bool, err error) {
	e := o.e
	f, err := e.flowOf(ctx, run)
	if err != nil {
		slog.WarnContext(ctx, "flowengine: a write event skipped a run whose flow cannot be read",
			"run", run.ID, "error", err)
		return false, false, nil
	}
	settle = joinWaits(run)
	for _, tok := range liveTokens(run) {
		if acted[tok] {
			settle = true
			continue
		}
		next, did, err := o.changeToken(ctx, w, changed, f, run, tok)
		if err != nil {
			return false, false, err
		}
		if next.ID == "" {
			return settle, true, nil
		}
		if did {
			acted[tok] = true
		}
		run, settle = next, settle || did
	}
	return settle, false, nil
}

// joinWaits reports whether run is forked and a branch has arrived at its
// join, so that the join may hold once the write commits.
func joinWaits(run flowrun.Run) bool {
	return forked(run) && slices.ContainsFunc(run.Branches, func(b flowrun.Branch) bool { return b.Arrived })
}

// liveTokens returns the tokens of run that can take an event: the run's
// own, or each branch that has not arrived at its join.
func liveTokens(run flowrun.Run) []string {
	if !forked(run) {
		return []string{""}
	}
	var out []string
	for _, b := range run.Branches {
		if !b.Arrived {
			out = append(out, b.Name)
		}
	}
	return out
}

// indexed is a `change` branch with its place among its state's.
type indexed struct {
	i int
	b flowcfg.Branch
}

// changeToken fires the event on token tok of run. It returns the run as
// stored after it acted, a zero run when the run was lost to another
// write, and whether it moved the token or kept the event on its mark.
func (o *OnChange) changeToken(ctx context.Context, w Write, changed map[string]bool, f *flowcfg.Flow,
	run flowrun.Run, tok string) (flowrun.Run, bool, error) {
	e := o.e
	inst, p, c, ok := slot(run, tok)
	if !ok {
		return run, false, nil
	}
	cur := flow.Read(f.Definition(), inst, run.CreatedAt)
	if cur.Stranded {
		return run, false, nil
	}
	bs := f.Branches(cur.State, flowcfg.EventChange)
	list := make([]indexed, len(bs))
	for i, b := range bs {
		list[i] = indexed{i: i, b: b}
	}
	d := flow.Decide(cur.State, list, flow.Rules[indexed]{
		Admits: func(x indexed) bool {
			return reads(x.b.When, changed) && (len(x.b.Roles) == 0 || slices.ContainsFunc(x.b.Roles, w.Holds))
		},
		Holds:  func(x indexed) (bool, error) { return e.g.holds(ctx, x.b.When, w.New) },
		Target: func(x indexed) string { return x.b.To },
	})
	switch d.Outcome {
	case flow.Moved:
	case flow.GuardFailed:
		slog.WarnContext(ctx, "flowengine: a change guard failed on a write", "run", run.ID, "state", cur.State)
		slog.DebugContext(ctx, "flowengine: change guard error", "run", run.ID, "error", d.Err)
		return run, false, nil
	default:
		return run, false, nil
	}
	now := e.now()
	by := clip(w.By)
	if c != nil && c.Generation == cur.Generation && c.By != by && now.Sub(c.At) < ClaimTTL {
		slog.InfoContext(ctx, "flowengine: a write event left a claimed step to its claimant",
			"run", run.ID, "state", cur.State)
		return run, false, nil
	}
	if p != nil {
		return o.keep(ctx, run, tok, inst, p, cur, flowrun.DeferredChange{Branch: d.Branch.i, By: by, At: now})
	}
	sctx := audit.WithFlowStep(ctx, audit.FlowStep{Flow: f.Name(), Run: string(run.ID), Step: cur.State, By: by})
	moved, err := e.take(sctx, f, run, tok, cur, d.Branch.b.Targets(), flowrun.Step{Via: flowrun.ViaChange, By: by},
		now)
	if isLost(err) {
		return flowrun.Run{}, false, nil
	}
	if err != nil {
		return run, false, fmt.Errorf("flowengine: move run %s on a write: %w", run.ID, err)
	}
	return moved, true, nil
}

// keep stores dc on the pending mark p of token tok, which is settling in
// the state cur. A mark that already keeps an event, or that names
// another state or generation, is left alone.
func (o *OnChange) keep(ctx context.Context, run flowrun.Run, tok string, inst flow.Instance, p *flowrun.Pending,
	cur flow.Current, dc flowrun.DeferredChange) (flowrun.Run, bool, error) {
	if p.Change != nil || p.State != cur.State || p.Generation != cur.Generation {
		slog.InfoContext(ctx, "flowengine: a write event was not kept: the settling step already keeps one",
			"run", run.ID, "state", cur.State)
		return run, false, nil
	}
	np := *p
	np.Change = &dc
	next, err := o.e.save(ctx, run, tok, inst, &np, nil, dc.At)
	if isLost(err) {
		return flowrun.Run{}, false, nil
	}
	if err != nil {
		return run, false, fmt.Errorf("flowengine: keep a write event on run %s: %w", run.ID, err)
	}
	return next, true, nil
}

// Settle settles run id now, as a due round would, also when it is
// forked: it writes the outputs it still owes, takes a kept event, and
// tests a waiting join. A run that is gone is left alone. A run that
// fails to settle is backed off, so a later round tries again.
func (o *OnChange) Settle(ctx context.Context, id flow.RunID) error {
	e := o.e
	run, err := e.d.Runs.Get(ctx, id)
	if errors.Is(err, flowrun.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	f, err := e.flowOf(ctx, run)
	if err != nil {
		return err
	}
	left, err := e.settle(ctx, f, run, "")
	if err != nil {
		e.backoff(ctx, left)
	}
	return err
}
