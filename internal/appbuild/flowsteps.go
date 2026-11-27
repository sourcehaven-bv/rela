package appbuild

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowedits"
	"github.com/Sourcehaven-BV/rela/internal/flowengine"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowview"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// MaxTaskCandidates caps the run headers one task list reads. Past it the
// list is served truncated (RR-KHWJI0P); a flow filter narrows it.
const MaxTaskCandidates = 500

// MaxRunList caps the run headers one run list reads. Past it the list is
// served truncated; a flow or status filter narrows it.
const MaxRunList = 1000

// maxRoleHolders caps the subjects one local-role query of the task list
// reads.
const maxRoleHolders = 5000

// errFlowInput answers a step that writes a property its state's form does
// not edit.
var errFlowInput = errors.New("flows: the step does not write that property")

// errFlowRequest answers a request the flow does not allow, such as a
// start on a subject of another type.
var errFlowRequest = errors.New("flows: request not allowed by the flow")

// errFlowBusy answers a cancel of a run that is still starting.
var errFlowBusy = errors.New("flows: run busy")

// errFlowCondition answers a manual start on a subject for which no
// manual start's `when:` holds.
var errFlowCondition = errors.New("flows: the start condition does not hold")

// classify wraps err in the [flowview] kind a transport maps to a status.
// Errors it does not know pass unchanged: the transport's own mapping of
// write errors (validation, field gate) applies to them.
func classify(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, flowengine.ErrRunNotFound), errors.Is(err, flowengine.ErrSubjectNotFound),
		errors.Is(err, flowengine.ErrUnknownFlow), errors.Is(err, entitymanager.ErrEntityNotFound),
		errors.Is(err, store.ErrNotFound):
		return fmt.Errorf("%w: %w", flowview.ErrNotFound, err)
	case errors.Is(err, flowengine.ErrStale), errors.Is(err, flowengine.ErrClaimed),
		errors.Is(err, flowengine.ErrSettling), errors.Is(err, flowengine.ErrNotPending),
		errors.Is(err, flowengine.ErrStranded), errors.Is(err, flowengine.ErrSubjectBusy),
		errors.Is(err, flowengine.ErrGuard), errors.Is(err, errFlowBusy), errors.Is(err, errFlowCondition),
		errors.Is(err, flowengine.ErrJobRunning), errors.Is(err, errStepForm):
		return fmt.Errorf("%w: %w", flowview.ErrConflict, err)
	case errors.Is(err, ErrFlowPermission), isFieldWriteError(err):
		return fmt.Errorf("%w: %w", flowview.ErrForbidden, err)
	case errors.Is(err, errFlowInput), errors.Is(err, errFlowRequest):
		return fmt.Errorf("%w: %w", flowview.ErrInvalid, err)
	}
	return err
}

// isFieldWriteError reports whether err is a field write refusal, such as
// another run's `outside_edits:` refusing a step's property (TKT-XI7IDBS).
func isFieldWriteError(err error) bool {
	var fw *affordances.FieldWriteError
	return errors.As(err, &fw)
}

// Flows lists the flows of the current flows.yaml. flows.yaml is
// configuration, not a secret, so the list is the same for everyone.
func (c *FlowControl) Flows() []flowview.Flow {
	flows := c.eng.Flows()
	out := make([]flowview.Flow, 0, len(flows))
	for _, f := range flows {
		out = append(out, flowview.Flow{
			Name: f.Name(), Label: f.Label(), Subject: f.Subject(), Version: f.Version(),
			Manual: len(manualPermissions(f)) > 0, PerSubject: string(f.PerSubject()), Diagram: f.MermaidAt(),
		})
	}
	return out
}

// caller is the per-operation view of the principal: its ACL request, or
// nil without acl.yaml, when every role counts as held (RR-Q4JAI21).
type caller struct {
	req   *acl.Request
	user  string
	roles map[string]map[string]bool // subject id -> roles held on it
}

// callerOf opens the operation's caller. Without an attached request it
// fails closed.
func (c *FlowControl) callerOf(ctx context.Context) (*caller, error) {
	cl := &caller{user: principal.From(ctx).User, roles: map[string]map[string]bool{}}
	if c.d == nil {
		return cl, nil
	}
	req, err := c.d.RequestFrom(ctx)
	if err != nil {
		return nil, err
	}
	cl.req = req
	return cl, nil
}

// holds returns whether the caller holds role on subjectID: globally or
// through a role relation. Without acl.yaml every role is held.
func (cl *caller) holds(ctx context.Context, subjectID string) func(string) bool {
	if cl.req == nil {
		return func(string) bool { return true }
	}
	set, ok := cl.roles[subjectID]
	if !ok {
		set = map[string]bool{}
		for _, a := range cl.req.ForEntity(ctx, "", subjectID) {
			set[a.Role] = true
		}
		cl.roles[subjectID] = set
	}
	return func(r string) bool { return set[r] }
}

// mayUpdate reports whether the caller may update the subject row: the
// manager's write decision, asked without a write and without a
// denied-write audit row, for the `complete` affordance (RR-B8ELMN5).
func (cl *caller) mayUpdate(ctx context.Context, subj *entity.Entity) bool {
	if cl.req == nil {
		return true
	}
	return cl.req.AuthorizeWrite(ctx, acl.WriteRequest{
		Op: acl.OpUpdate, Subject: acl.NewEntitySubject(subj.Type, subj.ID, subj.Face),
	}).Allow
}

// header reads the run header id through the row gate; a hidden header is
// the uniform not-found.
func (c *FlowControl) header(ctx context.Context, id flow.RunID) (*entity.Entity, error) {
	if id == "" {
		return nil, flowengine.ErrRunNotFound
	}
	hdr, err := c.rows.GetAddress(ctx, string(id))
	if err != nil {
		if isNotFound(err) {
			return nil, flowengine.ErrRunNotFound
		}
		return nil, err
	}
	if hdr.Type != entity.FlowRunType {
		return nil, flowengine.ErrRunNotFound
	}
	return hdr, nil
}

// Run returns run id as the caller may see it. Without a readable header
// the run is not found, whatever the run store holds.
func (c *FlowControl) Run(ctx context.Context, id flow.RunID) (flowview.Run, error) {
	cl, err := c.callerOf(ctx)
	if err != nil {
		return flowview.Run{}, err
	}
	hdr, err := c.header(ctx, id)
	if err != nil {
		return flowview.Run{}, classify(err)
	}
	var runs []flowrun.Run
	if run, rerr := c.eng.Run(ctx, id); rerr == nil {
		runs = append(runs, run)
	} else if !errors.Is(rerr, flowengine.ErrRunNotFound) {
		return flowview.Run{}, rerr
	}
	views, err := c.views(ctx, cl, []*entity.Entity{hdr}, runs, true)
	if err != nil {
		return flowview.Run{}, classify(err)
	}
	return views[0], nil
}

// SubjectRuns lists the runs on the subject at addr that the caller may
// see, newest first: the runs whose headers the row gate admits, read in
// one batch, so a run whose header is hidden stays hidden even though the
// subject is readable (RR-K0GSN1S).
func (c *FlowControl) SubjectRuns(ctx context.Context, addr string) ([]flowview.Run, error) {
	cl, err := c.callerOf(ctx)
	if err != nil {
		return nil, err
	}
	subj, err := c.read(ctx, addr)
	if err != nil {
		return nil, classify(err)
	}
	var ids []string
	for r, err := range c.rows.ListRelationsStrict(ctx, store.RelationQuery{
		To: subj.ID, Type: entity.CarriesRelation, Direction: store.DirectionIncoming,
	}) {
		if err != nil {
			return nil, err
		}
		ids = append(ids, r.From)
	}
	if len(ids) == 0 {
		return []flowview.Run{}, nil
	}
	var hdrs []*entity.Entity
	for e, err := range c.rows.ListEntities(ctx, store.EntityQuery{
		Type: entity.FlowRunType, IDs: ids, Faces: store.AllFaces(),
	}) {
		if err != nil {
			return nil, err
		}
		hdrs = append(hdrs, e)
	}
	runs, err := c.eng.SubjectRuns(ctx, subj.ID)
	if err != nil {
		return nil, err
	}
	views, err := c.views(ctx, cl, hdrs, runs, true)
	if err != nil {
		return nil, classify(err)
	}
	sort.SliceStable(views, func(i, j int) bool {
		if !views[i].Started.Equal(views[j].Started) {
			return views[i].Started.After(views[j].Started)
		}
		return views[i].ID < views[j].ID
	})
	return views, nil
}

// RunList lists the run headers the caller can read, newest first, one
// page of perPage. A non-empty flowName or status keeps the runs that
// match. The rows carry what the header holds and the subject's address;
// the run detail adds the waiting reason and the actions.
//
// The headers come from one gated store query, the subjects of the page
// from one edge query, so the cost does not grow with the number of runs.
func (c *FlowControl) RunList(ctx context.Context, flowName, status string, page, perPage int) (flowview.Runs, error) {
	if _, err := c.callerOf(ctx); err != nil {
		return flowview.Runs{}, err
	}
	eq := map[string]string{}
	if flowName != "" {
		eq[metamodel.FlowRunPropFlow] = flowName
	}
	// A run whose job step failed keeps its state's status on the header
	// (BUG-S5VV0N0), so `failed` also reads the runs in a job state and
	// keeps those whose job failed (BUG-53GP5F5).
	var anyOf []map[string]string
	switch status {
	case "":
	case string(flow.StatusFailed):
		anyOf = append(jobGroups(c.eng.Flows(), flowName),
			map[string]string{metamodel.FlowRunPropStatus: string(flow.StatusFailed)})
	default:
		eq[metamodel.FlowRunPropStatus] = status
	}
	hdrs, more, err := c.holders.Headers(ctx, visibility.HeaderQuery{
		EntityType: entity.FlowRunType, Equal: eq, AnyOf: anyOf, Faces: store.AllFaces(), Limit: MaxRunList,
		Order: []store.OrderSpec{{Property: metamodel.FlowRunPropStarted, Descending: true}},
	})
	if err != nil {
		return flowview.Runs{}, err
	}
	var failed map[flow.RunID]bool
	if status == string(flow.StatusFailed) {
		if hdrs, failed, err = c.keepFailed(ctx, hdrs); err != nil {
			return flowview.Runs{}, err
		}
	}
	out := flowview.Runs{Runs: []flowview.Run{}, Total: len(hdrs), Truncated: more}
	views := make([]flowview.Run, len(hdrs))
	for i, h := range hdrs {
		views[i] = headerView(&entity.Entity{ID: h.ID, Properties: h.Properties})
	}
	order := make([]int, len(hdrs))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := views[order[i]], views[order[j]]
		if !a.Started.Equal(b.Started) {
			return a.Started.After(b.Started)
		}
		return a.ID < b.ID
	})
	lo, hi := pageRange(len(order), page, perPage)
	order = order[lo:hi]
	pageHdrs := make([]store.EntityHeader, len(order))
	for i, k := range order {
		pageHdrs[i] = hdrs[k]
	}
	subjects, err := c.carried(ctx, pageHdrs)
	if err != nil {
		return flowview.Runs{}, err
	}
	if failed == nil {
		var unfinished []flow.RunID
		for _, k := range order {
			if !flow.Status(views[k].Status).Terminal() {
				unfinished = append(unfinished, flow.RunID(views[k].ID))
			}
		}
		if failed, err = c.failedJobs(ctx, unfinished); err != nil {
			return flowview.Runs{}, err
		}
	}
	for _, k := range order {
		v := c.labeled(views[k])
		v.Subject = subjects[v.ID]
		v.Active = !flow.Status(v.Status).Terminal()
		if failed[flow.RunID(v.ID)] {
			v.Status = string(flow.StatusFailed)
		}
		out.Runs = append(out.Runs, v)
	}
	if err := c.nameSubjects(ctx, out.Runs); err != nil {
		return flowview.Runs{}, err
	}
	return out, nil
}

// keepFailed returns the headers among hdrs of the runs that read as
// failed: the header says so, or the run waits on a failed job step, which
// failed names. The stored states come from one run store read.
func (c *FlowControl) keepFailed(ctx context.Context, hdrs []store.EntityHeader) ([]store.EntityHeader,
	map[flow.RunID]bool, error) {
	status := func(h store.EntityHeader) flow.Status {
		return flow.Status(propString(&entity.Entity{Properties: h.Properties}, metamodel.FlowRunPropStatus))
	}
	var unfinished []flow.RunID
	for _, h := range hdrs {
		if !status(h).Terminal() {
			unfinished = append(unfinished, flow.RunID(h.ID))
		}
	}
	failed, err := c.failedJobs(ctx, unfinished)
	if err != nil {
		return nil, nil, err
	}
	var out []store.EntityHeader
	for _, h := range hdrs {
		if status(h) == flow.StatusFailed || failed[flow.RunID(h.ID)] {
			out = append(out, h)
		}
	}
	return out, failed, nil
}

// failedJobs reads the stored state of the unfinished runs ids in one run
// store read and returns those that wait on a failed job step. Their
// headers still say working, so without this the list would disagree with
// the run's page (BUG-S5VV0N0).
func (c *FlowControl) failedJobs(ctx context.Context, ids []flow.RunID) (map[flow.RunID]bool, error) {
	if len(ids) == 0 {
		return map[flow.RunID]bool{}, nil
	}
	runs, err := c.eng.RunsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return c.eng.FlowCache().FailedJobs(ctx, runs), nil
}

// defaultFlowPage is the page size when the caller names none.
const defaultFlowPage = 50

// pageRange is the slice [lo, hi) of n items that page (from 1) of
// perPage items covers, with the default page size of 50 and the first
// page for values below 1. Page and perPage come from the caller, so it
// never multiplies them: a page past the end is empty, however large.
func pageRange(n, page, perPage int) (lo, hi int) {
	if perPage <= 0 {
		perPage = defaultFlowPage
	}
	if page < 1 {
		page = 1
	}
	if page-1 > n/perPage {
		return n, n
	}
	lo = min((page-1)*perPage, n)
	return lo, lo + min(perPage, n-lo)
}

// splitRuns indexes runs by id and sorts hdrs into the active runs, which
// have stored state among runs, and the headers of the finished ones.
func splitRuns(hdrs []*entity.Entity, runs []flowrun.Run) (byID map[flow.RunID]flowrun.Run,
	active []flowrun.Run, finished []store.EntityHeader) {
	byID = map[flow.RunID]flowrun.Run{}
	for _, r := range runs {
		byID[r.ID] = r
	}
	for _, h := range hdrs {
		if r, ok := byID[flow.RunID(h.ID)]; ok {
			active = append(active, r)
		} else {
			finished = append(finished, store.EntityHeader{ID: h.ID})
		}
	}
	return byID, active, finished
}

// views builds the run views of hdrs, which passed the row gate, with the
// stored state of the active runs among runs, and names their subjects.
// Detail adds the waiting reason, the timeline and the diagram.
func (c *FlowControl) views(ctx context.Context, cl *caller, hdrs []*entity.Entity, runs []flowrun.Run,
	detail bool) ([]flowview.Run, error) {
	out, err := c.unnamedViews(ctx, cl, hdrs, runs, detail)
	if err != nil {
		return nil, err
	}
	if err := c.nameSubjects(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// unnamedViews is views without the subject titles.
func (c *FlowControl) unnamedViews(ctx context.Context, cl *caller, hdrs []*entity.Entity, runs []flowrun.Run,
	detail bool) ([]flowview.Run, error) {
	byID, active, finished := splitRuns(hdrs, runs)
	// A finished run keeps only its header, so its subject comes from the
	// gated `_carries` edge, as in the run list (BUG-TVU9HRT).
	carried, err := c.carried(ctx, finished)
	if err != nil {
		return nil, err
	}
	xs, err := c.eng.Explain(ctx, active...)
	if err != nil {
		return nil, err
	}
	explained := map[flow.RunID]flowengine.Explanation{}
	for i, r := range active {
		explained[r.ID] = xs[i]
	}
	flows := c.eng.FlowCache()
	forms := c.forms.load(ctx)
	manages := &manageVerdicts{c: c}
	subjects := map[entity.Ref]*entity.Entity{}
	out := make([]flowview.Run, 0, len(hdrs))
	for _, h := range hdrs {
		v := c.labeled(headerView(h))
		run, ok := byID[flow.RunID(h.ID)]
		if !ok {
			v.Subject = carried[h.ID]
			if detail {
				c.addFinishedDetail(ctx, &v, h, flows, manages)
			}
			out = append(out, v)
			continue
		}
		x := explained[run.ID]
		v.Active, v.Subject, v.Generation, v.EnteredAt = true, run.Subject.String(), x.Generation, x.EnteredAt
		v.State, v.StateLabel = x.State, x.Label
		if x.Status != "" {
			v.Status = string(x.Status)
		}
		subj, err := c.subjectOf(ctx, subjects, run.Subject)
		if err != nil {
			return nil, err
		}
		if subj == nil {
			// The title can name the subject, as the address does.
			v.Subject, v.Title = "", v.FlowLabel
		}
		holds := cl.holds(ctx, run.Subject.ID)
		subject := subjectTypeOf(ctx, flows, run)
		w := waitingOf(x, holds, forms, subject)
		if detail {
			addDetail(w, x)
			addTimeline(ctx, &v, run, x, flows)
		}
		if subj == nil {
			// The guards were evaluated on a subject the caller cannot
			// read: their verdicts are its values.
			w.Branches = nil
		}
		v.Waiting = w
		mayStep := func(w *flowview.Waiting, x flowengine.Explanation) bool {
			return subj != nil && w.YouHold && w.FormProblem == "" && x.Pending == nil && !x.Stranded && !x.Arrived &&
				cl.mayUpdate(ctx, subj)
		}
		for _, bx := range x.Parallel {
			bw := waitingOf(bx, holds, forms, subject)
			if detail {
				addDetail(bw, bx)
			}
			if subj == nil {
				bw.Branches = nil
			}
			v.Parallel = append(v.Parallel, flowview.Place{
				Branch: bx.Branch, State: bx.State, StateLabel: bx.Label, Status: string(bx.Status),
				Generation: bx.Generation, EnteredAt: bx.EnteredAt, Arrived: bx.Arrived, Waiting: bw,
				Complete: mayStep(bw, bx),
			})
		}
		manage := manages.may(ctx, run.Subject.ID, c.runManagePermissions(ctx, flows, run))
		// The error text can hold subject values, so a manager who may not
		// read the subject does not get it either.
		withholdJobError(&v, manage, subj != nil)
		failedMessage(ctx, flows, run, &v)
		v.Actions = flowview.Actions{
			Complete: len(x.Parallel) == 0 && mayStep(w, x),
			Manage:   manage,
			Cancel:   manage,
			Retry:    manage && x.Pending != nil,
		}
		out = append(out, v)
	}
	return out, nil
}

// nameSubjects sets the subject title of each run and, when principals
// are person entities, the title of who caused each timeline entry, in
// one header batch. An actor is named only when its id reads as an
// entity of the person type: without person mapping a principal is a
// login, which may happen to equal an unrelated entity's id.
func (c *FlowControl) nameSubjects(ctx context.Context, runs []flowview.Run) error {
	addrs := make([]string, 0, len(runs))
	for _, r := range runs {
		addrs = append(addrs, r.Subject)
	}
	person := pilePersonType(c.d)
	if person != "" {
		for _, r := range runs {
			for _, e := range r.Log {
				addrs = append(addrs, e.By)
			}
		}
	}
	titles, err := c.typedTitles(ctx, addrs)
	if err != nil {
		return err
	}
	for i := range runs {
		if subj, ok := titles[runs[i].Subject]; ok {
			runs[i].SubjectTitle = subj.title
			runs[i].Title = c.viewTitle(runs[i])
		} else {
			// The subject is withheld from this reader. A stored title can
			// hold its values, so the run is named by its flow alone.
			runs[i].Subject, runs[i].SubjectTitle, runs[i].Title = "", "", flowOnlyTitle(runs[i])
		}
		for j := range runs[i].Log {
			e := &runs[i].Log[j]
			if t, ok := titles[e.By]; ok && person != "" && t.typ == person && t.title != e.By {
				e.ByTitle = t.title
			}
		}
	}
	return nil
}

// viewTitle is the title of v for its reader: the stored title of a flow
// with `title:`, or the flow's label and the subject's title as the reader
// reads it, also for a run started before titles were stored.
func (c *FlowControl) viewTitle(v flowview.Run) string {
	if f, ok := c.eng.Flow(v.Flow); ok && f.HasTitle() {
		return v.Title
	}
	if v.SubjectTitle == "" || v.SubjectTitle == v.Subject {
		return v.Title
	}
	return v.FlowLabel + ": " + v.SubjectTitle
}

// flowOnlyTitle names v by its flow, for a reader who may not read its
// subject.
func flowOnlyTitle(v flowview.Run) string {
	if v.FlowLabel != "" {
		return v.FlowLabel
	}
	return v.Flow
}

// subjectOf reads the subject at ref through the gate, once per
// operation: a subject the caller cannot read is nil, so it offers no
// step and its address is withheld.
func (c *FlowControl) subjectOf(ctx context.Context, seen map[entity.Ref]*entity.Entity,
	ref entity.Ref) (*entity.Entity, error) {
	if subj, ok := seen[ref]; ok {
		return subj, nil
	}
	subj, err := c.rows.GetAddress(ctx, ref.String())
	if err != nil {
		if !isNotFound(err) {
			return nil, err
		}
		subj = nil
	}
	seen[ref] = subj
	return subj, nil
}

// subjectTypeOf is the subject type of the flow version run follows, or ""
// when that version cannot be read; the run is then stranded.
func subjectTypeOf(ctx context.Context, flows *flowengine.FlowCache, run flowrun.Run) string {
	f, err := flows.Of(ctx, run)
	if err != nil {
		return ""
	}
	return f.Subject()
}

// stepInput checks props against the step form of x, read now, and returns
// their names in order.
func (c *FlowControl) stepInput(ctx context.Context, run flowrun.Run, x flowengine.Explanation,
	props map[string]any) ([]string, error) {
	f, err := c.eng.FlowCache().Of(ctx, run)
	if err != nil {
		return nil, err
	}
	fields, err := c.forms.load(ctx).fields(x.Form, f.Subject())
	if err != nil {
		return nil, err
	}
	return inputNames(props, fields)
}

// waitingOf is why a run with explanation x waits, for a caller whose
// roles on the subject holds reports. forms names the fields of its step,
// on a subject of the flow subject type subject.
func waitingOf(x flowengine.Explanation, holds func(string) bool, forms stepFormSet,
	subject string) *flowview.Waiting {
	w := &flowview.Waiting{
		Roles: x.WaitingFor, YouHold: slices.ContainsFunc(x.WaitingFor, holds),
		Form: x.Form, Stranded: x.Stranded,
	}
	if fields, err := forms.fields(x.Form, subject); err != nil {
		w.FormProblem = err.Error()
	} else {
		w.Fields = fields
	}
	if x.Pending != nil {
		w.Pending = &flowview.Pending{Failures: x.Pending.Failures, NextAttempt: x.Pending.NextAttempt}
		if j := x.Pending.Job; j != nil {
			w.Pending.Job = &flowview.Job{Phase: string(j.Phase), Failed: j.Failed, Attempts: j.Attempts,
				FailKind: string(j.FailKind), Error: j.Error}
		}
	}
	if x.Claim != nil {
		w.ClaimedBy = x.Claim.By
	}
	return w
}

// withholdJobError removes the job error from v and its branches unless
// the caller manages the run and reads its subject (BUG-53GP5F5).
func withholdJobError(v *flowview.Run, manage, readsSubject bool) {
	if manage && readsSubject {
		return
	}
	strip := func(w *flowview.Waiting) {
		if w != nil && w.Pending != nil && w.Pending.Job != nil {
			w.Pending.Job.Error = ""
		}
	}
	strip(v.Waiting)
	for i := range v.Parallel {
		strip(v.Parallel[i].Waiting)
	}
}

// failedMessage sets the `failed_message:` of run's flow on v when a job
// step of v failed for good. The rest of the time no client says it.
func failedMessage(ctx context.Context, flows *flowengine.FlowCache, run flowrun.Run, v *flowview.Run) {
	if !jobFailed(v) {
		return
	}
	if f, err := flows.Of(ctx, run); err == nil {
		v.FailedMessage = f.FailedMessage()
	}
}

// jobFailed reports that the job step of v, or of one of its parallel
// branches, failed for good.
func jobFailed(v *flowview.Run) bool {
	failed := func(w *flowview.Waiting) bool {
		return w != nil && w.Pending != nil && w.Pending.Job != nil && w.Pending.Job.Phase == string(flowrun.JobFailed)
	}
	if failed(v.Waiting) {
		return true
	}
	return slices.ContainsFunc(v.Parallel, func(p flowview.Place) bool { return failed(p.Waiting) })
}

// addDetail adds the timers and branches of x to w. A branch's fields are
// the fields of w's step form that its guard reads, in form order.
func addDetail(w *flowview.Waiting, x flowengine.Explanation) {
	for _, t := range x.Timers {
		w.Timers = append(w.Timers, flowview.Timer{Name: t.Name, Label: t.Label, To: t.To, Due: t.Due})
	}
	for _, b := range x.Branches {
		w.Branches = append(w.Branches, flowview.Branch{To: b.To, Fork: b.Fork, Label: b.Label, Roles: b.Roles,
			Guard: b.Guard, Holds: b.Holds, Failed: b.Failed, Fields: guardFields(w.Fields, b.Reads)})
	}
}

// guardFields returns the fields among fields that reads names, in fields
// order.
func guardFields(fields, reads []string) []string {
	var out []string
	for _, f := range fields {
		if slices.Contains(reads, f) {
			out = append(out, f)
		}
	}
	return out
}

// addTimeline adds the timeline and diagram of run to v. The diagram
// marks every state the run is in: one per live branch while it is
// forked.
func addTimeline(ctx context.Context, v *flowview.Run, run flowrun.Run, x flowengine.Explanation,
	flows *flowengine.FlowCache) {
	f, ferr := flows.Of(ctx, run)
	if ferr != nil {
		f = nil
	}
	v.Log = timeline(run.Log, f)
	if f == nil {
		return
	}
	current := []string{x.State}
	if len(x.Parallel) > 0 {
		current = current[:0]
		for _, bx := range x.Parallel {
			current = append(current, bx.State)
		}
	}
	v.Diagram = f.MermaidAt(current...)
}

// timeline is the run view of log, with the state labels of f.
//
// Nil f: accepted. The version no longer compiles, or its flow was
// removed, so states keep their names.
func timeline(log []flowrun.Step, f *flowcfg.Flow) []flowview.Event {
	out := make([]flowview.Event, 0, len(log))
	for _, st := range log {
		label := st.State
		if f != nil {
			label = stateLabel(f, st.State)
		}
		out = append(out, flowview.Event{At: st.At, State: st.State, Label: label, From: st.From,
			Via: string(st.Via), Timer: st.Timer, By: st.By, Branch: st.Branch})
	}
	return out
}

// addFinishedDetail adds to v, the view of the finished run with header h,
// the timeline the header kept, the diagram at its final state, and the
// manage action. Cancel and retry stay off: the run has ended.
//
// An unreadable log is logged and shows no timeline, so one bad header does
// not fail the page. A version that no longer compiles takes its labels from
// the current flow, and its diagram only when the final state is still in
// it. A forked run gets no diagram: the header keeps one state, not the
// branches.
func (c *FlowControl) addFinishedDetail(ctx context.Context, v *flowview.Run, h *entity.Entity,
	flows *flowengine.FlowCache, manages *manageVerdicts) {
	log, err := flowrun.ParseLogBody(h.Content)
	if err != nil {
		slog.WarnContext(ctx, "flows: a finished run's log is unreadable", "run", h.ID, "error", err)
		log = nil
	}
	run := flowrun.Run{ID: flow.RunID(h.ID), Flow: v.Flow, Version: v.Version}
	f, ferr := flows.Of(ctx, run)
	if ferr != nil {
		f, _ = c.eng.Flow(v.Flow)
	}
	v.Log = timeline(log, f)
	if f != nil && !forked(log) {
		if _, ok := f.State(v.State); ok {
			v.Diagram = f.MermaidAt(v.State)
		}
	}
	// The subject is empty when the caller cannot read it; then nobody
	// manages the run here.
	ref, err := entity.ParseRef(v.Subject)
	if v.Subject == "" || err != nil {
		return
	}
	v.Actions.Manage = manages.may(ctx, ref.ID, c.runManagePermissions(ctx, flows, run))
}

// forked reports whether log has a step on a branch.
func forked(log []flowrun.Step) bool {
	for _, st := range log {
		if st.Branch != "" {
			return true
		}
	}
	return false
}

// manageVerdicts remembers, within one views call, whether the caller
// manages runs on a subject, so a subject's many runs cost one check.
type manageVerdicts struct {
	c    *FlowControl
	seen map[string]bool
}

// may reports whether the caller holds perms on the subject subjectID.
func (m *manageVerdicts) may(ctx context.Context, subjectID string, perms []string) bool {
	key := subjectID + "\x00" + strings.Join(perms, "\x00")
	if ok, hit := m.seen[key]; hit {
		return ok
	}
	if m.seen == nil {
		m.seen = map[string]bool{}
	}
	ok := m.c.authorize(ctx, subjectID, perms) == nil
	m.seen[key] = ok
	return ok
}

// runManagePermissions are the permissions that cancel or retry run: the
// manage permissions of the run's version, or of the current flow for a
// run whose version no longer compiles. A run keeps the rule of the version
// it started on, so adding `manage:` applies to runs started after it.
func (c *FlowControl) runManagePermissions(ctx context.Context, flows *flowengine.FlowCache, run flowrun.Run) []string {
	if f, err := flows.Of(ctx, run); err == nil {
		return managePermissions(f)
	}
	if f, ok := c.eng.Flow(run.Flow); ok {
		return managePermissions(f)
	}
	return nil
}

// stateLabel is the label of state name in f, or the name when f has no
// such state.
func stateLabel(f *flowcfg.Flow, name string) string {
	if st, ok := f.State(name); ok && st.Label != "" {
		return st.Label
	}
	return name
}

// labeled sets the flow and state labels of a header view from the
// current flow of that name. A finished run keeps only its header, so the
// current flow is the best source of its labels.
func (c *FlowControl) labeled(v flowview.Run) flowview.Run {
	if f, ok := c.eng.Flow(v.Flow); ok {
		v.FlowLabel = f.Label()
		v.StateLabel = stateLabel(f, v.State)
	}
	return v
}

// headerView is the part of a run view its header holds.
func headerView(h *entity.Entity) flowview.Run {
	state := propString(h, metamodel.FlowRunPropState)
	return flowview.Run{
		ID: h.ID, Title: propString(h, metamodel.FlowRunPropTitle), Flow: propString(h, metamodel.FlowRunPropFlow),
		Version: propString(h, metamodel.FlowRunPropVersion),
		State:   state, StateLabel: state, Status: propString(h, metamodel.FlowRunPropStatus),
		Started:   propTime(h, metamodel.FlowRunPropStarted),
		Completed: propTime(h, metamodel.FlowRunPropCompleted),
	}
}

// propTime reads a datetime property, which a backend may hand back as
// RFC 3339 text.
func propTime(e *entity.Entity, key string) time.Time {
	switch v := e.Properties[key].(type) {
	case time.Time:
		return v.UTC()
	case string:
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// Complete does the step of run id at generation gen for the caller: it
// writes props, which must all be fields of the state's `form:`, on the
// subject as the caller, and then fires the state's change event.
//
// The checks run in this order, so nothing about a hidden run leaks
// (RR-K0GSN1S): the bound identity, the header's row gate (not found), the
// run store, the generation (conflict), the properties (invalid), the
// waiting role (forbidden), the subject's read gate and the manager's
// update authorization. The step is then claimed (RR-LGRBAP4), written
// through the field gate with the flow step on ctx, and moved.
func (c *FlowControl) Complete(ctx context.Context, id flow.RunID, gen int64,
	props map[string]any) (flowview.Completed, error) {
	out, err := c.complete(ctx, id, gen, props)
	return out, classify(err)
}

func (c *FlowControl) complete(ctx context.Context, id flow.RunID, gen int64,
	props map[string]any) (flowview.Completed, error) {
	cl, err := c.callerOf(ctx)
	if err != nil {
		return flowview.Completed{}, err
	}
	if _, err = c.header(ctx, id); err != nil {
		return flowview.Completed{}, err
	}
	run, err := c.eng.Run(ctx, id)
	if err != nil {
		if errors.Is(err, flowengine.ErrRunNotFound) {
			return flowview.Completed{}, fmt.Errorf("%w: the run has ended", flowengine.ErrStale)
		}
		return flowview.Completed{}, err
	}
	branch, ok := flowengine.StepAt(run, gen)
	if !ok {
		return flowview.Completed{}, flowengine.ErrStale
	}
	xs, err := c.eng.Explain(ctx, run)
	if err != nil {
		return flowview.Completed{}, err
	}
	x, ok := stepOf(xs[0], branch)
	if !ok {
		return flowview.Completed{}, flowengine.ErrStale
	}
	if x.Stranded {
		return flowview.Completed{}, flowengine.ErrStranded
	}
	names, err := c.stepInput(ctx, run, x, props)
	if err != nil {
		return flowview.Completed{}, err
	}
	holds := cl.holds(ctx, run.Subject.ID)
	if !slices.ContainsFunc(x.WaitingFor, holds) {
		if len(x.WaitingFor) == 0 {
			return flowview.Completed{}, fmt.Errorf("%w: the state waits for nobody", ErrFlowPermission)
		}
		return flowview.Completed{}, fmt.Errorf("%w: needs role %v", ErrFlowPermission, x.WaitingFor)
	}
	subj, err := c.rows.GetAddress(ctx, run.Subject.String())
	if err != nil {
		return flowview.Completed{}, err
	}
	grant, err := c.writer.AuthorizeUpdate(ctx, subj)
	if err != nil {
		return flowview.Completed{}, err
	}
	if cerr := c.eng.Claim(ctx, id, gen, cl.user); cerr != nil {
		return flowview.Completed{}, cerr
	}
	wctx := audit.WithFlowStep(ctx, audit.FlowStep{Flow: run.Flow, Run: string(id), Step: x.State, By: cl.user})
	var warnings []flowview.FieldNote
	if len(props) > 0 {
		// The step's own write: this run's outside_edits do not narrow it,
		// another run's on the subject still do (TKT-XI7IDBS).
		pctx := withStepWrite(flowedits.WithinRun(wctx, id, x.State), id)
		res, perr := grant.PatchEntity(pctx, run.Subject.String(), entity.Patch{Properties: props})
		if perr != nil {
			c.eng.ReleaseClaim(ctx, id, gen, cl.user)
			return flowview.Completed{}, perr
		}
		warnings = fieldNotes(res, names)
	}
	changed, err := c.eng.Change(wctx, id, gen, cl.user, holds)
	out := flowview.Completed{Written: names, Moved: changed.Moved, State: changed.State, Finished: changed.Finished,
		Warnings: warnings}
	if err == nil && changed.Blocked {
		out.Problem = flowview.ProblemBlocked
	}
	if err == nil || (len(props) == 0 && !changed.Moved) {
		return out, err
	}
	// The write has landed, so the step is not refused: say how the run
	// fared instead.
	slog.WarnContext(ctx, "flowengine: a step was written but the run did not follow", "run", id, "error", err)
	return followProblem(out, err)
}

// followProblem is out with the problem of a step whose write landed but
// whose run did not follow with err. An err it cannot place is returned.
func followProblem(out flowview.Completed, err error) (flowview.Completed, error) {
	switch {
	case out.Moved:
		out.Problem = flowview.ProblemSettling
	case errors.Is(err, flowengine.ErrGuard):
		out.Problem = flowview.ProblemGuard
	case errors.Is(err, flowengine.ErrStale), errors.Is(err, flowengine.ErrClaimed):
		out.Problem = flowview.ProblemMoved
	default:
		return out, err
	}
	return out, nil
}

// Retry settles a run whose step failed, now. It needs a permission that
// may cancel the run: the people who manage runs, not those who wait.
func (c *FlowControl) Retry(ctx context.Context, id flow.RunID) error {
	if _, err := c.callerOf(ctx); err != nil {
		return err
	}
	if _, err := c.header(ctx, id); err != nil {
		return classify(err)
	}
	run, err := c.eng.Run(ctx, id)
	if err != nil {
		return classify(err)
	}
	if err := c.authorize(ctx, run.Subject.ID, c.runManagePermissions(ctx, c.eng.FlowCache(), run)); err != nil {
		return classify(err)
	}
	return classify(c.eng.Retry(byCaller(ctx), id))
}

// fieldNotes returns the warnings of res on the properties the step sent,
// in res's order. A warning's path is a JSON pointer to the property.
//
// Nil: res accepted, returns nil.
func fieldNotes(res *entity.UpdateResult, sent []string) []flowview.FieldNote {
	if res == nil {
		return nil
	}
	var out []flowview.FieldNote
	for _, w := range res.Warnings {
		name, ok := strings.CutPrefix(w.Path, "/properties/")
		if !ok {
			continue
		}
		name = strings.ReplaceAll(strings.ReplaceAll(name, "~1", "/"), "~0", "~")
		if slices.Contains(sent, name) {
			out = append(out, flowview.FieldNote{Field: name, Message: w.Detail})
		}
	}
	return out
}

// inputNames returns the names of props, sorted, or errFlowInput for a
// name that is not one of fields, the properties the step's form edits.
func inputNames(props map[string]any, fields []string) ([]string, error) {
	names := make([]string, 0, len(props))
	for k := range props {
		if !slices.Contains(fields, k) {
			return nil, fmt.Errorf("%w: %q", errFlowInput, k)
		}
		names = append(names, k)
	}
	slices.Sort(names)
	return names, nil
}

// stepOf returns the explanation of the step of branch in x: x itself for
// the run's own step. A step of a forked run is a branch's step: its state
// names what it writes and who it waits for.
func stepOf(x flowengine.Explanation, branch string) (flowengine.Explanation, bool) {
	if branch == "" {
		return x, true
	}
	i := slices.IndexFunc(x.Parallel, func(bx flowengine.Explanation) bool { return bx.Branch == branch })
	if i < 0 {
		return flowengine.Explanation{}, false
	}
	return x.Parallel[i], true
}

// taskState is one (flow, state) the task list asks the store for.
type taskState struct {
	flow  *flowcfg.Flow
	state flowcfg.State
	// join marks the join of a fork whose branches wait for a role the
	// caller may hold: the header of a forked run names it.
	join bool
}

// taskStates returns the (flow, state) pairs that wait for a role the
// caller may hold, and the header groups the store reads candidates by.
// A state in a parallel branch is not on any header: its fork's join is,
// so the join's group stands in for it.
func taskStates(flows []*flowcfg.Flow, flowName string, cl *caller) (
	states map[[2]string]taskState, groups []map[string]string) {
	states = map[[2]string]taskState{}
	group := func(f *flowcfg.Flow, state string) {
		groups = append(groups, map[string]string{
			metamodel.FlowRunPropFlow: f.Name(), metamodel.FlowRunPropState: state,
		})
	}
	for _, f := range flows {
		if flowName != "" && f.Name() != flowName {
			continue
		}
		for _, st := range f.States() {
			if len(st.WaitingFor()) == 0 || !slices.ContainsFunc(st.WaitingFor(), cl.mayHold) {
				continue
			}
			key := [2]string{f.Name(), st.Name}
			had, grouped := states[key]
			states[key] = taskState{flow: f, state: st, join: had.join}
			fk, _, inBranch := f.Region(st.Name)
			if !inBranch {
				if !grouped {
					group(f, st.Name)
				}
				continue
			}
			key = [2]string{f.Name(), fk.Join}
			_, grouped = states[key]
			js, _ := f.State(fk.Join)
			states[key] = taskState{flow: f, state: js, join: true}
			if !grouped {
				group(f, fk.Join)
			}
		}
	}
	return states, groups
}

// forkedRuns reads the stored runs of the candidate headers that name a
// join, in one run store read: their branches say which steps wait.
func (c *FlowControl) forkedRuns(ctx context.Context, hdrs []store.EntityHeader,
	states map[[2]string]taskState) (map[flow.RunID]flowrun.Run, error) {
	var ids []flow.RunID
	for _, h := range hdrs {
		hv := headerView(&entity.Entity{ID: h.ID, Properties: h.Properties})
		if states[[2]string{hv.Flow, hv.State}].join {
			ids = append(ids, flow.RunID(h.ID))
		}
	}
	out := map[flow.RunID]flowrun.Run{}
	if len(ids) == 0 {
		return out, nil
	}
	runs, err := c.eng.RunsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, r := range runs {
		if len(r.Branches) > 0 {
			out[r.ID] = r
		}
	}
	return out, nil
}

// Tasks lists the runs that wait for a role the caller holds on their
// subject, oldest first, one page of them. The store reads the candidates
// in one query (RR-KHWJI0P): headers the row gate admits, waiting for
// input, in a (flow, state) whose waiting roles the caller holds globally
// or could hold through a role relation. Then one query reads their
// subjects and one query per (subject type, local role) the subjects on
// which the caller holds that role, so the cost does not grow with the
// runs. A non-empty flowName keeps one flow.
//
// Candidates are matched on the current flows.yaml: a run on an older
// version in a state the current version renamed is not listed. The
// header can lag the run store (RR-9H6KV01); completing revalidates.
//
// It leaves Failed empty: [FlowControl.FailedTasks] reads those, so the
// next-action source and MCP do not pay for them.
func (c *FlowControl) Tasks(ctx context.Context, flowName string, page, perPage int) (flowview.Tasks, error) {
	cl, err := c.callerOf(ctx)
	if err != nil {
		return flowview.Tasks{}, err
	}
	states, groups := taskStates(c.eng.Flows(), flowName, cl)
	out := flowview.Tasks{Tasks: []flowview.Task{}}
	if len(groups) == 0 {
		return out, nil
	}
	hdrs, more, err := c.holders.Headers(ctx, visibility.HeaderQuery{
		EntityType: entity.FlowRunType,
		Equal:      map[string]string{metamodel.FlowRunPropStatus: string(flow.StatusInputRequired)},
		AnyOf:      groups, Faces: store.AllFaces(), Limit: MaxTaskCandidates,
		Order: []store.OrderSpec{{Property: metamodel.FlowRunPropStarted}},
	})
	if err != nil {
		return flowview.Tasks{}, err
	}
	out.Truncated = more
	subjects, err := c.carried(ctx, hdrs)
	if err != nil {
		return flowview.Tasks{}, err
	}
	local, err := c.localHolders(ctx, cl, hdrs, states)
	if err != nil {
		return flowview.Tasks{}, err
	}
	forked, err := c.forkedRuns(ctx, hdrs, states)
	if err != nil {
		return flowview.Tasks{}, err
	}
	for _, h := range hdrs {
		hv := headerView(&entity.Entity{ID: h.ID, Properties: h.Properties})
		subj, ok := subjects[h.ID]
		if !ok {
			continue
		}
		held := func(ts taskState) bool {
			return slices.ContainsFunc(ts.state.WaitingFor(), func(r string) bool {
				return cl.holdsGlobally(r) || local[[2]string{r, subj}]
			})
		}
		task := flowview.Task{Run: h.ID, Subject: subj, Started: hv.Started}
		ts := states[[2]string{hv.Flow, hv.State}]
		if run, stored := forked[flow.RunID(h.ID)]; ts.join && stored {
			// The header names the join: the steps are the branches'.
			for _, b := range run.Branches {
				bs := states[[2]string{hv.Flow, b.Instance.State}]
				if b.Arrived || bs.flow == nil || !held(bs) {
					continue
				}
				t := task
				t.Flow, t.FlowLabel, t.State, t.StateLabel, t.Branch = bs.flow.Name(), bs.flow.Label(), bs.state.Name,
					bs.state.Label, b.Name
				out.Tasks = append(out.Tasks, t)
			}
			continue
		}
		if ts.flow == nil || !held(ts) {
			continue
		}
		task.Flow, task.FlowLabel, task.State, task.StateLabel = ts.flow.Name(), ts.flow.Label(), ts.state.Name,
			ts.state.Label
		out.Tasks = append(out.Tasks, task)
	}
	sort.SliceStable(out.Tasks, func(i, j int) bool {
		if !out.Tasks[i].Started.Equal(out.Tasks[j].Started) {
			return out.Tasks[i].Started.Before(out.Tasks[j].Started)
		}
		return out.Tasks[i].Run < out.Tasks[j].Run
	})
	out.Total = len(out.Tasks)
	lo, hi := pageRange(len(out.Tasks), page, perPage)
	out.Tasks = out.Tasks[lo:hi]
	addrs := make([]string, len(out.Tasks))
	for i, t := range out.Tasks {
		addrs[i] = t.Subject
	}
	titles, err := c.titles(ctx, addrs)
	if err != nil {
		return flowview.Tasks{}, err
	}
	for i := range out.Tasks {
		out.Tasks[i].SubjectTitle = titles[out.Tasks[i].Subject]
	}
	return out, nil
}

// jobGroups returns the header groups under which a run can wait on a
// job step of the flows: the job state, or for a job state in a parallel
// branch the join, which the header of a forked run names.
func jobGroups(flows []*flowcfg.Flow, flowName string) []map[string]string {
	var groups []map[string]string
	seen := map[[2]string]bool{}
	for _, f := range flows {
		if flowName != "" && f.Name() != flowName {
			continue
		}
		for _, st := range f.States() {
			if _, ok := st.Job(); !ok {
				continue
			}
			state := st.Name
			if fk, _, inBranch := f.Region(st.Name); inBranch {
				state = fk.Join
			}
			if key := [2]string{f.Name(), state}; !seen[key] {
				seen[key] = true
				groups = append(groups, map[string]string{
					metamodel.FlowRunPropFlow: f.Name(), metamodel.FlowRunPropState: state,
				})
			}
		}
	}
	return groups
}

// FailedTasks lists the unfinished runs whose job step failed for good
// and that the caller manages, newest first (BUG-53GP5F5). A non-empty
// flowName keeps one flow. The store reads the candidates, the runs in a
// job state, in one gated header query, their subjects in one edge query
// and their stored state in one run store read. The manage check runs once
// per subject of a failed run only; a failed run is rare and waits for an
// operator. more reports that more candidates exist than one call reads.
func (c *FlowControl) FailedTasks(ctx context.Context, flowName string) (tasks []flowview.Task, more bool,
	err error) {
	tasks = []flowview.Task{}
	groups := jobGroups(c.eng.Flows(), flowName)
	if len(groups) == 0 {
		return tasks, false, nil
	}
	hdrs, more, err := c.holders.Headers(ctx, visibility.HeaderQuery{
		EntityType: entity.FlowRunType, AnyOf: groups, Faces: store.AllFaces(), Limit: MaxTaskCandidates,
		Order: []store.OrderSpec{{Property: metamodel.FlowRunPropStarted, Descending: true}},
	})
	if err != nil || len(hdrs) == 0 {
		return tasks, more, err
	}
	subjects, err := c.carried(ctx, hdrs)
	if err != nil {
		return nil, false, err
	}
	ids := make([]flow.RunID, 0, len(hdrs))
	started := map[flow.RunID]time.Time{}
	for _, h := range hdrs {
		if _, ok := subjects[h.ID]; ok {
			ids = append(ids, flow.RunID(h.ID))
			started[flow.RunID(h.ID)] = headerView(&entity.Entity{ID: h.ID, Properties: h.Properties}).Started
		}
	}
	if len(ids) == 0 {
		return tasks, more, nil
	}
	runs, err := c.eng.RunsByID(ctx, ids)
	if err != nil {
		return nil, false, err
	}
	flows := c.eng.FlowCache()
	places := flows.FailedJobPlaces(ctx, runs)
	manages := &manageVerdicts{c: c}
	for _, run := range runs {
		place, ok := places[run.ID]
		if !ok || !manages.may(ctx, run.Subject.ID, c.runManagePermissions(ctx, flows, run)) {
			continue
		}
		t := flowview.Task{Run: string(run.ID), Flow: run.Flow, FlowLabel: run.Flow, State: place.State,
			StateLabel: place.State, Subject: subjects[string(run.ID)], Started: started[run.ID], Branch: place.Branch}
		if f, ok := c.eng.Flow(run.Flow); ok {
			t.FlowLabel, t.StateLabel = f.Label(), stateLabel(f, place.State)
		}
		tasks = append(tasks, t)
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		if !tasks[i].Started.Equal(tasks[j].Started) {
			return tasks[i].Started.After(tasks[j].Started)
		}
		return tasks[i].Run < tasks[j].Run
	})
	addrs := make([]string, len(tasks))
	for i, t := range tasks {
		addrs[i] = t.Subject
	}
	titles, err := c.titles(ctx, addrs)
	if err != nil {
		return nil, false, err
	}
	for i := range tasks {
		tasks[i].SubjectTitle = titles[tasks[i].Subject]
	}
	return tasks, more, nil
}

// mayHold reports whether the caller holds role globally or could hold it
// on some entity through a role relation.
func (cl *caller) mayHold(role string) bool {
	if cl.req == nil || cl.holdsGlobally(role) {
		return true
	}
	_, ok := cl.req.LocalRolePredicates(role)
	return ok
}

// holdsGlobally reports whether the caller holds role everywhere.
func (cl *caller) holdsGlobally(role string) bool {
	if cl.req == nil {
		return true
	}
	return slices.ContainsFunc(cl.req.Globals().Attributions,
		func(a acl.RoleAttribution) bool { return a.Role == role })
}

// carried maps each header to the address of the subject it carries, in
// one gated edge query.
func (c *FlowControl) carried(ctx context.Context, hdrs []store.EntityHeader) (map[string]string, error) {
	out := map[string]string{}
	if len(hdrs) == 0 {
		return out, nil
	}
	ids := make([]string, len(hdrs))
	for i, h := range hdrs {
		ids[i] = h.ID
	}
	for r, err := range c.rows.ListRelationsStrict(ctx, store.RelationQuery{
		Type: entity.CarriesRelation, EntityIDs: ids, Direction: store.DirectionOutgoing,
	}) {
		if err != nil {
			return nil, err
		}
		out[r.From] = r.To
	}
	return out, nil
}

// titles maps each subject address in addrs to its display title, read
// in one gated header batch ([metamodel.Metamodel.WholeDisplayTitle]). An
// address the caller cannot read, or that does not parse, is left out.
func (c *FlowControl) titles(ctx context.Context, addrs []string) (map[string]string, error) {
	typed, err := c.typedTitles(ctx, addrs)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(typed))
	for a, t := range typed {
		out[a] = t.title
	}
	return out, nil
}

// typedTitle is an entity's display title with its type.
type typedTitle struct{ title, typ string }

// typedTitles is titles with each entity's type.
func (c *FlowControl) typedTitles(ctx context.Context, addrs []string) (map[string]typedTitle, error) {
	out := map[string]typedTitle{}
	byRef := map[entity.Ref]string{}
	for _, a := range addrs {
		if a == "" {
			continue
		}
		if ref, err := entity.ParseRef(a); err == nil {
			byRef[ref] = a
		}
	}
	if len(byRef) == 0 {
		return out, nil
	}
	got, err := c.rows.ResolveHeadersErr(ctx, slices.Collect(maps.Keys(byRef)))
	if err != nil {
		return nil, err
	}
	for ref, rh := range got {
		if !rh.Served() {
			continue
		}
		addr := byRef[ref]
		if c.meta == nil {
			out[addr] = typedTitle{title: addr, typ: rh.Header.Type}
			continue
		}
		out[addr] = typedTitle{title: c.meta.WholeDisplayTitle(addr, rh.Header.Type, rh.Header.Properties),
			typ: rh.Header.Type}
	}
	return out, nil
}

// localHolders returns the (role, subject id) pairs among hdrs' subjects
// on which the caller holds a waiting role through a role relation: one
// store query per (subject type, role) that the caller does not hold
// globally.
func (c *FlowControl) localHolders(ctx context.Context, cl *caller, hdrs []store.EntityHeader,
	states map[[2]string]taskState) (map[[2]string]bool, error) {
	out := map[[2]string]bool{}
	if cl.req == nil || len(hdrs) == 0 {
		return out, nil
	}
	need := map[[2]string]bool{} // (subject type, role)
	for _, ts := range states {
		for _, r := range ts.state.WaitingFor() {
			if !cl.holdsGlobally(r) {
				need[[2]string{ts.flow.Subject(), r}] = true
			}
		}
	}
	gq, ok := c.raw.(store.GraphQueryer)
	if !ok {
		return out, nil // coverage-ignore: every backend answers graph queries
	}
	for key := range need {
		preds, ok := cl.req.LocalRolePredicates(key[1])
		if !ok {
			continue
		}
		// One query per predicate: a conditional role relation has its own
		// (TKT-TKSCSBT), and the role is held where any of them matches.
		for i := range preds {
			q := store.GraphQuery{
				EntityType: key[0], HasInbound: &preds[i], Faces: store.AllFaces(), Limit: maxRoleHolders,
			}
			for h, err := range store.GraphQueryHeaders(ctx, gq, q) {
				if err != nil {
					return nil, err
				}
				out[[2]string{key[1], h.ID}] = true
			}
		}
	}
	return out, nil
}
