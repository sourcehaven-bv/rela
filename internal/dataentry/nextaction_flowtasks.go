package dataentry

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
)

// flowTaskSource answers a `source: flow_tasks` next-action source. It is
// its own type so the App does not grow a method per source kind.
type flowTaskSource struct{ app *App }

// candidates yields the reader's waiting flow tasks as the
// candidates of a `source: flow_tasks` next-action source, oldest first.
//
// The tasks come from [FlowService.Tasks], the call behind the Flow tasks
// page, `GET /_flows/tasks` and MCP `list_flow_tasks`. So the suggestion
// and the list apply the same row gate and the same role rules (global
// roles and role relations), and this adds no rule of its own.
//
// Each candidate's entity is the task's subject, read in one gated batch
// per subject type and field-redacted like a query candidate. Its title
// fills {subject}. A subject this read does not return still yields its
// task, with the address as the title, so the hint names exactly the tasks
// the list names.
//
// A request with no bound identity contributes nothing, like a per-user
// condition (see [nextaction.ErrIdentityRequired]): the hint is advisory
// and must not fail the page it sits on.
func (f flowTaskSource) candidates(
	ctx context.Context, meta *metamodel.Metamodel, src dataentryconfig.NextActionSource, want nextaction.Want,
) ([]nextaction.Candidate, error) {
	a := f.app
	if a.flows == nil || meta == nil {
		return nil, nil
	}
	svc := a.flows.svc
	subjectType := map[string]string{}
	for _, f := range svc.Flows() {
		subjectType[f.Name] = f.Subject
	}
	tasks, err := flowTasksFor(ctx, svc, src.Flows, subjectType)
	if errors.Is(err, acl.ErrNoIdentity) {
		slog.Debug("nextaction: flow_tasks source skipped, the request has no identity")
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("next-action flow tasks: %w", err)
	}
	tasks = firstTasks(tasks, want)
	subjects, err := f.subjects(ctx, tasks, subjectType)
	if err != nil {
		return nil, err
	}
	out := make([]nextaction.Candidate, 0, len(tasks))
	for _, t := range tasks {
		typ := subjectType[t.Flow]
		e := subjects[t.Subject]
		title := t.Subject
		if e != nil {
			title = safeDisplayTitle(meta, e)
		} else {
			e = &entityPkg.Entity{ID: subjectID(t.Subject), Type: typ}
		}
		out = append(out, nextaction.Candidate{Entity: e, Task: &nextaction.FlowTask{
			Run:         t.Run,
			Branch:      t.Branch,
			State:       t.State,
			Flow:        cmp.Or(t.FlowLabel, t.Flow),
			Step:        cmp.Or(t.StateLabel, t.State),
			Subject:     title,
			SubjectType: typ,
			Address:     t.Subject,
			Link:        flowTaskLink(typ, t.Subject, t.Run),
		}})
	}
	return out, nil
}

// flowTasksFor reads the caller's tasks of the named flows, or of every
// flow when names is empty, oldest first. One call per flow, because the
// task call reads a bounded number of candidates and a flow filter narrows
// them. A name flows.yaml does not declare is logged and skipped: flows.yaml
// and data-entry.yaml reload separately, so load time cannot check it.
func flowTasksFor(
	ctx context.Context, svc FlowService, names []string, subjectType map[string]string,
) ([]flowview.Task, error) {
	if len(names) == 0 {
		names = []string{""}
	}
	var out []flowview.Task
	for _, name := range names {
		if _, ok := subjectType[name]; name != "" && !ok {
			slog.Warn("nextaction: flow_tasks source names an undeclared flow", "flow", name)
			continue
		}
		got, err := svc.Tasks(ctx, name, 1, maxFlowTaskCandidates)
		if err != nil {
			return nil, err
		}
		out = append(out, got.Tasks...)
	}
	slices.SortStableFunc(out, func(x, y flowview.Task) int {
		if c := x.Started.Compare(y.Started); c != 0 {
			return c
		}
		return cmp.Compare(x.Run, y.Run)
	})
	return out, nil
}

// maxFlowTaskCandidates is the page of tasks one flow contributes. The task
// call computes the whole list anyway, so a larger page costs no extra
// read; [firstTasks] then keeps what the engine uses.
const maxFlowTaskCandidates = 500

// firstTasks keeps the leading want.First tasks, plus the one on screen
// (want.Keep is its subject id) wherever it falls.
func firstTasks(tasks []flowview.Task, want nextaction.Want) []flowview.Task {
	if want.First <= 0 || len(tasks) <= want.First {
		return tasks
	}
	out := slices.Clone(tasks[:want.First])
	keep := func(t flowview.Task) bool { return subjectID(t.Subject) == want.Keep }
	if want.Keep == "" || slices.ContainsFunc(out, keep) {
		return out
	}
	for _, t := range tasks[want.First:] {
		if keep(t) {
			return append(out, t)
		}
	}
	return out
}

// subjects reads the tasks' subjects through the caller's read gate:
// one id-bounded read per subject type, never one per task. The result is
// keyed by the task's subject address, and each entity is redacted. An
// address that names no face takes the first readable face of the id.
func (f flowTaskSource) subjects(
	ctx context.Context, tasks []flowview.Task, subjectType map[string]string,
) (map[string]*entityPkg.Entity, error) {
	a := f.app
	byType := map[string][]string{}
	addrs := map[string]entityPkg.Address{}
	for _, t := range tasks {
		addr, err := entityPkg.ParseAddress(t.Subject)
		typ := subjectType[t.Flow]
		if err != nil || typ == "" {
			continue
		}
		if _, seen := addrs[t.Subject]; !seen {
			addrs[t.Subject] = addr
			byType[typ] = append(byType[typ], addr.ID())
		}
	}
	rows := map[string][]*entityPkg.Entity{}
	for typ, ids := range byType {
		rqr := readGateFromContext(ctx).ReadQuery(ctx, typ)
		got, err := scopedEntities(ctx, a.Services(), rqr, scopeRequest{
			Type: typ, Faces: rqr.Faces, ByIDs: true, IDs: ids,
		})
		if err != nil {
			return nil, fmt.Errorf("next-action flow task subjects %q: %w", typ, err)
		}
		for _, e := range got {
			rows[e.ID] = append(rows[e.ID], e)
		}
	}
	var picked []*entityPkg.Entity
	out := map[string]*entityPkg.Entity{}
	for key, addr := range addrs {
		named, isNamed := addr.Named()
		for _, e := range rows[addr.ID()] {
			if !isNamed || e.Face == named.Face {
				out[key] = e
				picked = append(picked, e)
				break
			}
		}
	}
	ctx = primeVerdicts(ctx, a.fieldResolver, picked)
	aff := a.affordances
	for addr, e := range out {
		out[addr] = redactedForSuggestion(ctx, aff, e)
	}
	return out, nil
}

// subjectID is the entity id of a task's subject address, the id a
// candidate's entity and its key carry whether or not the subject was read.
func subjectID(address string) string {
	if addr, err := entityPkg.ParseAddress(address); err == nil {
		return addr.ID()
	}
	return address
}

// flowTaskLink is the subject page with the run's step in view: the entity
// page focuses the run that `?run=` names in its Flows panel.
func flowTaskLink(typ, subject, run string) string {
	if typ == "" || subject == "" {
		return ""
	}
	return "/entity/" + url.PathEscape(typ) + "/" + url.PathEscape(subject) + "?run=" + url.QueryEscape(run)
}
