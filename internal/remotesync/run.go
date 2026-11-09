package remotesync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
)

// Writer is the write path a run uses. *entitymanager.Manager implements
// it; the manager authorizes, validates and audits every write as the
// principal on ctx.
type Writer interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
	PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	CreateRelation(ctx context.Context, key entity.RelationKey, opts entity.RelationOptions) (*entity.Relation, error)
	UpdateRelation(ctx context.Context, key entity.RelationKey, opts entity.RelationOptions) (*entity.Relation, error)
	DeleteRelation(ctx context.Context, key entity.RelationKey) error
}

// RawReader reads the local rows a run merges into. It is a write-prep
// read, like the entity manager's own: the run diffs against the stored
// row, so a field the pull's principal could not see is never written
// blind. The manager still authorizes every write. store.Store implements
// it.
type RawReader interface {
	store.GraphQueryer
	GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error)
	GetRelation(ctx context.Context, k entity.RelationKey) (*entity.Relation, error)
}

// Deps are the collaborators of one run.
type Deps struct {
	// Source is the remote.
	Source Source
	Meta   *metamodel.Metamodel
	Writer Writer
	Store  RawReader
	// State holds the ledger.
	State  state.KV
	Logger *slog.Logger
	// RunID names the run in the log lines of its deletes.
	RunID string
	// DiffDeadline bounds the diff; zero means the reconcile default.
	DiffDeadline time.Duration
	// Now is the clock of the gone guard's delete window; nil means
	// time.Now.
	Now func() time.Time
}

func (d Deps) validate() error {
	switch {
	case d.Source == nil:
		return errors.New("remotesync: a source is required")
	case d.Meta == nil:
		return errors.New("remotesync: a schema is required")
	case d.Writer == nil:
		return errors.New("remotesync: a writer is required")
	case d.Store == nil:
		return errors.New("remotesync: a store is required")
	case d.State == nil:
		return errors.New("remotesync: a state store is required")
	case d.Logger == nil:
		return errors.New("remotesync: a logger is required")
	}
	return nil
}

// Options are the operator's choices for one run.
type Options struct {
	// DryRun reports what the run would do and writes nothing, not even
	// the ledger.
	DryRun bool
	// AllowGone confirms that up to this many linked rows may be gone,
	// above the remote's max_gone, and lifts the empty-type guard for them.
	AllowGone int
}

// ErrGoneGuard reports a run the gone guard refused. Nothing was applied.
var ErrGoneGuard = errors.New("remotesync: refused by the gone guard")

// Dial returns the source of remote p, authenticated with credential, the
// value [Remote.ReadCredential] returned. transport may be nil.
func Dial(p Remote, credential string, transport http.RoundTripper) (Source, error) {
	header := p.AuthHeader
	if header == "" {
		header = "Authorization"
	}
	auth, err := httpremote.Header(header, credential)
	if err != nil {
		// The error names the rule, never the value.
		return nil, fmt.Errorf("remotesync: %s: %w", p.credentialSource(), err)
	}
	r, err := httpremote.New(httpremote.Config{
		BaseURL: p.URL, Auth: auth, Transport: transport, AllowInsecureHTTP: p.AllowInsecureHTTP,
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// ReadCredential reads the credential p names. getenv reads an environment
// variable; secret reads the global section of the project's secrets.
// Errors name the source, never the value.
//
// Nil: getenv and secret are accepted; the source that needs a nil one
// fails.
func (p Remote) ReadCredential(getenv func(string) string, secret func(string) (string, error)) (string, error) {
	if p.Credential.IsZero() {
		return "", errors.New("remotesync: credential: no source")
	}
	v, err := p.Credential.Read(getenv, secret)
	if err != nil {
		return "", fmt.Errorf("remotesync: %s: %w", p.credentialSource(), err)
	}
	return v, nil
}

// credentialSource names the credential's source for errors.
func (p Remote) credentialSource() string {
	switch p.Credential.Kind() {
	case httpremote.CredentialSecret:
		return "credential secret " + p.Credential.Secret
	case httpremote.CredentialFile:
		return "credential file " + p.Credential.File
	case httpremote.CredentialEnv:
		return "credential env $" + p.Credential.Env
	}
	return "credential"
}

// Run pulls m's remote once. The caller stamps the pull's principal on ctx
// and binds it, holds the run lock of the remote, and bounds ctx.
//
// The returned report is valid with an error too: a failed run saves the
// ledger with what it applied, so the next run does not apply it again.
func Run(ctx context.Context, m *Mapping, d Deps, o Options) (Report, error) {
	if m == nil {
		return Report{}, errors.New("remotesync: a mapping is required")
	}
	rep := Report{Remote: m.Name, RunID: d.RunID, DryRun: o.DryRun}
	if err := d.validate(); err != nil {
		return rep, err
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	l, err := loadLedger(ctx, d.State, m)
	if err != nil {
		return rep, err
	}
	r := &runner{m: m, d: d, o: o, l: l, rep: &rep}
	if prepErr := r.prepare(ctx); prepErr != nil {
		return rep, prepErr
	}
	diff, err := r.diff(ctx)
	if err != nil {
		return rep, err
	}
	gone, err := r.guard(diff)
	if err != nil {
		return rep, err
	}
	runErr := r.apply(ctx, diff, gone)
	if !o.DryRun {
		if err := l.save(context.WithoutCancel(ctx), d.State, m); err != nil {
			return rep, errors.Join(runErr, err)
		}
	}
	if runErr == nil && (rep.Entities.Failed > 0 || rep.Relations.Failed > 0) {
		runErr = fmt.Errorf("remotesync: %d rows failed", rep.Entities.Failed+rep.Relations.Failed)
	}
	return rep, runErr
}

// runner holds the state of one run.
type runner struct {
	m   *Mapping
	d   Deps
	o   Options
	l   *ledger
	rep *Report

	plurals map[string]string
	// byID maps a source entity id to its ledger row. Source ids are
	// unique across types, as rela ids are.
	byID map[string]*ledgerRow
	// fetched counts the rows fetched against max_fetch, and read the
	// bytes they took against max_bytes.
	fetched int
	read    int64
	// cur is the source row being pulled. An issue raised while it is set
	// holds the row, so later runs take it last (see [ledgerDoc.Held]).
	cur *heldRow
}

// heldRow names a source row at one source hash.
type heldRow struct {
	key  string
	hash canonical.Digest
}

// hold marks the current row as stopped without being applied.
func (r *runner) hold() {
	if r.cur != nil {
		r.l.held[r.cur.key] = r.cur.hash
	}
}

// budget reports whether the run may fetch one more row. Over max_fetch or
// max_bytes the row is deferred and counted as waiting.
func (r *runner) budget(c *Counts) bool {
	if r.fetched >= r.m.Remote.maxFetch() || r.read >= r.m.Remote.maxBytes() {
		c.Deferred++
		r.rep.Waiting++
		return false
	}
	r.fetched++
	return true
}

// Get reads through the source and counts the bytes against max_bytes.
func (r *runner) Get(ctx context.Context, path string, limit int64) ([]byte, error) {
	raw, err := r.d.Source.Get(ctx, path, limit)
	r.read += int64(len(raw))
	return raw, err
}

func (r *runner) prepare(ctx context.Context) error {
	p, err := plurals(ctx, r.d.Source, r.m)
	if err != nil {
		return err
	}
	r.plurals = p
	r.byID = map[string]*ledgerRow{}
	for _, row := range r.l.rows {
		if row.Kind == "E" {
			_, id, _ := strings.Cut(row.Address, "/")
			r.byID[id] = row
		}
	}
	return nil
}

func (r *runner) diff(ctx context.Context) (reconclient.Result, error) {
	r.l.buildIndex()
	rows := r.m.Remote.maxRows()
	differ, err := reconclient.New(local{r.l}, r.d.Source, reconclient.Options{
		MaxRemoteElements: rows, MaxLocalElements: rows, Deadline: r.d.DiffDeadline,
	})
	if err != nil {
		return reconclient.Result{}, err
	}
	res, err := differ.Diff(ctx, reconclient.Scope{
		Entities: sortedKeys(r.m.Entities), Relations: sortedKeys(r.m.Relations),
	})
	if err != nil {
		return res, fmt.Errorf("remotesync: compare with the source: %w", err)
	}
	for _, s := range res.Skipped {
		r.rep.Skipped = append(r.rep.Skipped, s.Type)
	}
	return res, nil
}

// guard picks the linked rows the source no longer lists, drops the other
// gone rows from the ledger except the dropped ones, and refuses the run when the gone rows are
// too many or empty a type. It writes nothing.
func (r *runner) guard(diff reconclient.Result) ([]*ledgerRow, error) {
	skipped := map[string]bool{}
	for _, s := range diff.Skipped {
		if s.Kind == reconclient.KindRelation {
			skipped[s.Type] = true
		}
	}
	var gone, forget []*ledgerRow
	goneOf := map[string]int{}
	for _, el := range diff.OnlyLocal {
		row := r.l.get(reconcileKind(el.Kind), el.Address)
		if row == nil || (row.Kind == "R" && skipped[row.Type]) {
			continue
		}
		goneOf[row.Kind+row.Type]++
		switch row.State {
		case stateLinked:
			gone = append(gone, row)
		case stateDropped:
			// Kept, so a local delete stays one when the source lists
			// the row again.
		default:
			forget = append(forget, row)
		}
	}
	r.rep.Gone = len(gone)
	if len(gone) > 0 && len(gone) > r.o.AllowGone {
		limit := r.m.Remote.maxGone()
		if len(gone) > limit {
			r.rep.Refused = fmt.Sprintf("%d linked rows are gone from the source, over max_gone %d; "+
				"confirm with --allow-gone %d", len(gone), limit, len(gone))
			return nil, fmt.Errorf("%w: %s", ErrGoneGuard, r.rep.Refused)
		}
		if recent := r.l.recentDeletes(r.d.Now()); r.m.Remote.onGone() == OnGoneDelete && recent+len(gone) > limit {
			r.rep.Refused = fmt.Sprintf("%d linked rows are gone from the source and %d were deleted in the last %s, "+
				"over max_gone %d; confirm with --allow-gone %d", len(gone), recent, deleteWindow, limit, len(gone))
			return nil, fmt.Errorf("%w: %s", ErrGoneGuard, r.rep.Refused)
		}
		if t, ok := r.emptied(diff, goneOf, skipped); ok {
			r.rep.Refused = fmt.Sprintf("the source lists no rows of %s any more; confirm with --allow-gone %d",
				t, len(gone))
			return nil, fmt.Errorf("%w: %s", ErrGoneGuard, r.rep.Refused)
		}
	}
	if !r.o.DryRun {
		for _, row := range forget {
			r.l.remove(row)
		}
	}
	return gone, nil
}

// emptied returns a type that had linked rows and that the source now lists
// no rows of. A type the source stopped serving, through its ACL or a
// broken mapping, looks like that; deleting its rows on that evidence is the
// mass delete the guard exists for.
func (r *runner) emptied(diff reconclient.Result, goneOf map[string]int, skipped map[string]bool) (string, bool) {
	listed, linked := map[string]int{}, map[string]int{}
	for kind, rows := range r.l.index {
		for _, row := range rows {
			k := string(kind) + row.Type
			listed[k]++
			if row.State == stateLinked {
				linked[k]++
			}
		}
	}
	for _, el := range diff.OnlyRemote {
		listed[string(reconcileKind(el.Kind))+elementType(el)]++
	}
	for _, k := range sortedKeys(linked) {
		if linked[k] > 0 && listed[k]-goneOf[k] <= 0 && (k[0] != 'R' || !skipped[k[1:]]) {
			kind := "entity type"
			if k[0] == 'R' {
				kind = "relation type"
			}
			return fmt.Sprintf("%s %q", kind, k[1:]), true
		}
	}
	return "", false
}

// apply pulls the new and changed rows, entities first so relations find
// their endpoints, then handles the gone rows, relations first so an
// entity's pulled relations are gone before it is.
func (r *runner) apply(ctx context.Context, diff reconclient.Result, gone []*ledgerRow) error {
	ents, rels := r.work(diff)
	pull := func(kind reconcile.Kind, it workItem, f func(context.Context, string, canonical.Digest) error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.cur = &heldRow{key: rowKey(kind, it.addr), hash: it.hash}
		defer func() { r.cur = nil }()
		return f(ctx, it.addr, it.hash)
	}
	for _, it := range ents {
		if err := pull(reconcile.KindEntity, it, r.pullEntity); err != nil {
			return err
		}
	}
	for _, it := range rels {
		if err := pull(reconcile.KindRelation, it, r.pullRelation); err != nil {
			return err
		}
	}
	for _, kind := range []string{"R", "E"} {
		for _, row := range gone {
			if err := ctx.Err(); err != nil {
				return err
			}
			switch {
			case row.Kind != kind:
			case kind == "R":
				r.goneRelation(ctx, row)
			default:
				r.goneEntity(ctx, row)
			}
		}
	}
	return ctx.Err()
}

// workItem is one new or changed source row.
type workItem struct {
	addr string
	hash canonical.Digest
}

// work splits the new and changed rows into entities and relations, each in
// pull order, and forgets the held rows the diff no longer lists. Rows held
// at the hash the source still has go last, in address order, so they
// cannot use up the fetch budget ahead of the others.
func (r *runner) work(diff reconclient.Result) (ents, rels []workItem) {
	add := func(kind reconclient.Kind, it workItem) {
		if kind == reconclient.KindRelation {
			rels = append(rels, it)
		} else {
			ents = append(ents, it)
		}
	}
	for _, el := range diff.OnlyRemote {
		add(el.Kind, workItem{el.Address, el.Hash})
	}
	for _, ch := range diff.Differing {
		add(ch.Kind, workItem{ch.Address, ch.Remote})
	}
	listed := map[string]bool{}
	order := func(kind reconcile.Kind, items []workItem) {
		held := func(it workItem) int {
			k := rowKey(kind, it.addr)
			listed[k] = true
			if h, ok := r.l.held[k]; ok && h == it.hash {
				return 1
			}
			return 0
		}
		slices.SortFunc(items, func(a, b workItem) int {
			if c := held(a) - held(b); c != 0 {
				return c
			}
			return strings.Compare(a.addr, b.addr)
		})
	}
	order(reconcile.KindEntity, ents)
	order(reconcile.KindRelation, rels)
	for k := range r.l.held {
		if !listed[k] {
			delete(r.l.held, k)
		}
	}
	return ents, rels
}

func reconcileKind(k reconclient.Kind) reconcile.Kind {
	if k == reconclient.KindRelation {
		return reconcile.KindRelation
	}
	return reconcile.KindEntity
}

// elementType returns the type an address names.
func elementType(el reconclient.Element) string {
	if el.Kind == reconclient.KindRelation {
		_, typ, _, _ := splitRelationAddress(el.Address)
		return typ
	}
	typ, _, _ := strings.Cut(el.Address, "/")
	return typ
}

// issue reports a row that was not applied, and holds the current row.
func (r *runner) issue(kind, address, local, reason string, fields ...string) {
	r.hold()
	r.rep.Issues = append(r.rep.Issues, Issue{
		Kind: kind, Address: address, Local: local, Reason: reconclient.Sanitize(reason), Fields: fields,
	})
}

// fetchFailed classifies a source error: a denied read ends the run, a row
// that went away waits for the next run, an invalid answer rejects the row
// (rejected is true), and anything else fails the row.
func (r *runner) fetchFailed(c *Counts, kind, address string, err error) (rejected bool, _ error) {
	switch {
	case errors.Is(err, httpremote.ErrDenied):
		return false, fmt.Errorf("remotesync: the source refused a read: %w", err)
	case errors.Is(err, httpremote.ErrNotFound):
		c.Deferred++
		return false, nil
	case errors.Is(err, errProtocol), errors.Is(err, reconclient.ErrProtocol):
		c.Rejected++
		r.issue(IssueRejected, address, "", err.Error())
		return true, nil
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return false, err
	}
	c.Failed++
	r.issue(IssueFailed, address, "", kind+" fetch: "+err.Error())
	return false, nil
}

// theirsValue is one owned field as the source has it.
type theirsValue struct {
	f FieldMapping
	// v is the canonical value; nil means cleared.
	v any
}

// theirs reads the owned fields of a fetched row. A field the source
// redacted is not reported, so it is never cleared. An absent field is
// cleared only when the source says what it redacted; without that, absent
// is not reported either.
func theirs(fields []FieldMapping, props map[string]any, content *string, redacted *[]string) ([]theirsValue, error) {
	var out []theirsValue
	for _, f := range fields {
		if redacted != nil && slices.Contains(*redacted, f.Remote) {
			continue
		}
		var raw any
		present := false
		if f.Remote == syncmerge.ContentField && f.Local.Kind == syncmerge.KindContent {
			if content != nil {
				raw, present = *content, true
			}
		} else {
			raw, present = props[f.Remote]
		}
		switch {
		case present:
			v, err := syncmerge.Canonical(f.Local, plain(raw))
			if err != nil {
				return nil, err
			}
			if f.Local.Kind == syncmerge.KindContent && v == nil {
				v = ""
			}
			out = append(out, theirsValue{f: f, v: v})
		case redacted != nil:
			var v any
			if f.Local.Kind == syncmerge.KindContent {
				v = ""
			}
			out = append(out, theirsValue{f: f, v: v})
		}
	}
	return out, nil
}

func oursValue(f syncmerge.Field, e *entity.Entity) any {
	if f.Kind == syncmerge.KindContent {
		return e.Content
	}
	return e.Properties[f.Name]
}

// merged is the three-way outcome of one row.
type merged struct {
	props     map[string]any
	unset     []string
	content   *string
	base      map[string]string
	conflicts []string
}

func (m merged) empty() bool { return len(m.props) == 0 && len(m.unset) == 0 && m.content == nil }

// merge compares each reported owned field on digests: the same value on
// both sides is unchanged; a local value still at the base takes theirs; a
// source value still at the base keeps the local value; anything else, and
// any difference without a base, is a conflict.
func merge(fields []FieldMapping, base map[string]string, ours *entity.Entity, th []theirsValue) merged {
	out := merged{props: map[string]any{}, base: map[string]string{}}
	for _, f := range fields {
		if b, ok := base[f.baseKey()]; ok {
			out.base[f.baseKey()] = b
		}
	}
	for _, t := range th {
		f, k := t.f.Local, t.f.baseKey()
		dt, do := syncmerge.Digest(f, t.v), syncmerge.Digest(f, oursValue(f, ours))
		b, hasBase := base[k]
		switch {
		case do == dt:
			out.base[k] = dt
		case !hasBase:
			out.conflicts = append(out.conflicts, f.Name)
		case b == do:
			out.set(f, t.v)
			out.base[k] = dt
		case b == dt:
			// The local edit stands; the base stays at the source value.
		default:
			out.conflicts = append(out.conflicts, f.Name)
		}
	}
	return out
}

func (m *merged) set(f syncmerge.Field, v any) {
	switch {
	case f.Kind == syncmerge.KindContent:
		s, _ := v.(string)
		m.content = &s
	case v == nil:
		m.unset = append(m.unset, f.Name)
	default:
		m.props[f.Name] = v
	}
}

// maxAttempts bounds the tries of one row's write against concurrent
// edits.
const maxAttempts = 2

func (r *runner) pullEntity(ctx context.Context, addr string, hash canonical.Digest) error {
	c := &r.rep.Entities
	typ, id, _ := strings.Cut(addr, "/")
	em := r.m.Entities[typ]
	row := r.l.get(reconcile.KindEntity, addr)
	if em == nil { // coverage-ignore: invariant: the diff lists only the types in its scope
		return fmt.Errorf("remotesync: the diff listed %s outside the mapping", addr)
	}
	if strings.Contains(id, entity.StateRefSeparator) {
		c.Rejected++
		r.issue(IssueRejected, addr, "", "the source row has a face; faced rows are not pulled")
		r.record(row, "E", typ, addr, hash, "", stateRejected, nil)
		return nil
	}
	if row != nil && row.State == stateDropped {
		c.Dropped++
		row.Hash = hash
		return nil
	}
	if !r.budget(c) {
		return nil
	}
	e, err := fetchEntity(ctx, r, r.plurals[typ], typ, id)
	if err != nil {
		rejected, runErr := r.fetchFailed(c, "entity", addr, err)
		if rejected {
			r.record(row, "E", typ, addr, hash, localOf(row), stateRejected, baseOf(row))
		}
		return runErr
	}
	th, err := theirs(em.Fields, e.Properties, e.Content, e.Redacted)
	if err != nil {
		c.Rejected++
		r.issue(IssueRejected, addr, "", err.Error())
		r.record(row, "E", typ, addr, hash, localOf(row), stateRejected, baseOf(row))
		return nil
	}
	for range maxAttempts {
		if !r.applyEntity(ctx, em, row, addr, id, hash, th) {
			return nil
		}
		row = r.l.get(reconcile.KindEntity, addr)
	}
	c.Failed++
	r.issue(IssueFailed, addr, localOf(row), "the local row kept changing during the run")
	return nil
}

// applyEntity merges one fetched source row into its local row. It returns
// true when the local row changed under the write and the row should be
// tried again. Every other outcome is counted in the report.
func (r *runner) applyEntity(
	ctx context.Context, em *EntityMapping, row *ledgerRow, addr, id string, hash canonical.Digest, th []theirsValue,
) bool {
	c := &r.rep.Entities
	ours, err := r.locate(ctx, row, id)
	if err != nil {
		c.Failed++
		r.issue(IssueFailed, addr, localOf(row), err.Error())
		return false
	}
	if ours == nil && row != nil && row.State == stateLinked {
		// Someone deleted the local row; the pull does not recreate it, so
		// the ledger keeps no base for it.
		c.Dropped++
		r.record(row, "E", em.Source, addr, hash, row.Local, stateDropped, nil)
		return false
	}
	if ours == nil {
		r.create(ctx, em, row, addr, id, hash, th)
		return false
	}
	if ours.Type != em.Local {
		c.Conflicts++
		r.issue(IssueConflict, addr, ours.ID, fmt.Sprintf("the ref is held by a %s, not a %s", ours.Type, em.Local))
		return false
	}
	linked := row != nil && row.State == stateLinked && row.Local == ours.ID
	// The ledger never knew this local row: it was there before the pull.
	adopted := row == nil || row.Local != ours.ID || row.Adopted
	mg := merge(em.Fields, baseOf(row), ours, th)
	if len(mg.conflicts) > 0 {
		c.Conflicts++
		r.issue(IssueConflict, addr, ours.ID, "both sides changed owned fields", mg.conflicts...)
		return false
	}
	switch {
	case mg.empty() && linked:
		c.Unchanged++
	case mg.empty():
		c.Adopted++
	case r.o.DryRun:
		c.Updated++
		return false
	default:
		_, err := r.d.Writer.PatchEntity(ctx, ours.ID, entity.Patch{
			Properties: mg.props, MetaUnset: mg.unset, Content: mg.content,
			ExpectedVersion: string(store.VersionOf(ours)),
		})
		if _, conflict := errors.AsType[*store.VersionConflictError](err); conflict {
			return true
		}
		if err != nil {
			r.writeFailed(c, addr, ours.ID, err)
			return false
		}
		if linked {
			c.Updated++
		} else {
			c.Adopted++
		}
	}
	r.record(row, "E", em.Source, addr, hash, ours.ID, stateLinked, mg.base).adopt(ours.Face, adopted)
	return false
}

// locate returns the local row a source row maps to: the ledger's local id,
// else the entity holding the source id in its ref (a renamed row, or one
// created by a run whose ledger was not saved). Nil means there is none.
func (r *runner) locate(ctx context.Context, row *ledgerRow, id string) (*entity.Entity, error) {
	if row != nil && row.Local != "" {
		e, err := r.d.Store.GetEntity(ctx, row.localRef())
		switch {
		case err == nil:
			return e, nil
		case !errors.Is(err, store.ErrNotFound):
			return nil, err
		}
	}
	found, err := r.findByRef(ctx, id)
	if err != nil {
		return nil, err
	}
	switch len(found) {
	case 0:
		return nil, nil //nolint:nilnil // nil means no local row
	case 1:
		return found[0], nil
	}
	return nil, fmt.Errorf("%d local entities hold ref %q of system %q", len(found), id, r.m.System)
}

// findByRef returns the entities whose external ref of the remote's system,
// in any type declaring it, holds id. The system is unique per (system, id)
// across those types, so there is at most one unless the store was written
// around the manager.
func (r *runner) findByRef(ctx context.Context, id string) ([]*entity.Entity, error) {
	var out []*entity.Entity
	for _, h := range metamodel.ExternalRefProps(r.d.Meta, r.m.System) {
		q := store.GraphQuery{
			EntityType: h.Type,
			Props:      []store.PropPredicate{{Property: h.Property, Op: store.PropKeyEqual, Key: "id", Value: id}},
			Faces:      store.AllFaces(),
		}
		for hdr, err := range store.GraphQueryHeaders(ctx, r.d.Store, q) {
			if err != nil {
				return nil, fmt.Errorf("find external ref: %w", err)
			}
			e, err := r.d.Store.GetEntity(ctx, entity.Ref{ID: hdr.ID, Face: hdr.Face})
			if errors.Is(err, store.ErrNotFound) {
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("find external ref: %w", err)
			}
			if got, _ := metamodel.ExternalRefID(e.Properties[h.Property]); got == id {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// create writes a new local row for a source row: a generated id, the
// owned fields, and the ref holding the source id.
func (r *runner) create(
	ctx context.Context, em *EntityMapping, row *ledgerRow, addr, id string, hash canonical.Digest, th []theirsValue,
) {
	c := &r.rep.Entities
	e := &entity.Entity{Type: em.Local, Properties: map[string]any{
		em.Ref: metamodel.ExternalRefValue{ID: id}.Map(),
	}}
	base := map[string]string{}
	for _, t := range th {
		base[t.f.baseKey()] = syncmerge.Digest(t.f.Local, t.v)
		switch {
		case t.f.Local.Kind == syncmerge.KindContent:
			e.Content, _ = t.v.(string)
		case t.v != nil:
			e.Properties[t.f.Local.Name] = t.v
		}
	}
	if r.o.DryRun {
		c.Created++
		r.record(row, "E", em.Source, addr, hash, "", stateLinked, base)
		return
	}
	res, err := r.d.Writer.CreateEntity(ctx, e, entity.CreateOptions{WriteExternalRefs: true})
	switch {
	case err == nil:
	case uniqueOn(err, em.Ref):
		// Another entity took the ref since the lookup.
		c.Deferred++
		r.hold()
		return
	case rejectedWrite(err):
		c.Rejected++
		r.issue(IssueRejected, addr, "", err.Error())
		r.record(row, "E", em.Source, addr, hash, "", stateRejected, nil)
		return
	default:
		c.Failed++
		r.issue(IssueFailed, addr, "", err.Error())
		return
	}
	c.Created++
	r.record(row, "E", em.Source, addr, hash, res.Entity.ID, stateLinked, base).adopt(res.Entity.Face, false)
}

// uniqueOn reports a unique violation on prop. A violation that names no
// property is taken to be on prop, the external ref the create sets.
func uniqueOn(err error, prop string) bool {
	if u, ok := errors.AsType[store.UniquePropertyError](err); ok {
		return u.Property == "" || u.Property == prop
	}
	if v, ok := errors.AsType[*entitymanager.ValidationError](err); ok {
		for _, e := range v.Errors {
			if e.Type == metamodel.ValidationErrorUnique && (e.Property == "" || e.Property == prop) {
				return true
			}
		}
	}
	return false
}

// rejectedWrite reports a write the local schema refused: a validation
// error, a unique value, a relation the schema does not allow between the
// endpoints, or a relation over its cardinality. It is not tried again
// until the source row changes.
func rejectedWrite(err error) bool {
	_, invalid := errors.AsType[*entitymanager.ValidationError](err)
	_, badRelation := errors.AsType[*entitymanager.InvalidRelationError](err)
	return invalid || badRelation || entitymanager.IsUniqueViolation(err) ||
		errors.Is(err, entitymanager.ErrCardinalityExceeded)
}

func (r *runner) writeFailed(c *Counts, addr, local string, err error) {
	if rejectedWrite(err) {
		c.Rejected++
		r.issue(IssueRejected, addr, local, err.Error())
		return
	}
	c.Failed++
	r.issue(IssueFailed, addr, local, err.Error())
}

// record stores the outcome of a source row in the ledger, releases it
// from the held rows, and returns the row. A dry run records nothing and
// returns nil; it only notes the outcome of an entity row, so the relations
// that follow count as a real run would.
func (r *runner) record(
	row *ledgerRow, kind, typ, addr string, hash canonical.Digest, localID, st string, base map[string]string,
) *ledgerRow {
	if r.o.DryRun {
		if kind == "E" {
			_, id, _ := strings.Cut(addr, "/")
			r.byID[id] = &ledgerRow{Kind: kind, Type: typ, Address: addr, Local: localID, State: st}
		}
		return nil
	}
	if row == nil {
		row = &ledgerRow{Kind: kind, Type: typ, Address: addr}
		r.l.put(row)
	}
	row.Hash, row.Local, row.State, row.Base = hash, localID, st, base
	delete(r.l.held, rowKey(row.kind(), addr))
	if kind == "E" {
		_, id, _ := strings.Cut(addr, "/")
		r.byID[id] = row
	}
	return row
}

func localOf(row *ledgerRow) string {
	if row == nil {
		return ""
	}
	return row.Local
}

func baseOf(row *ledgerRow) map[string]string {
	if row == nil {
		return nil
	}
	return maps.Clone(row.Base)
}

func (r *runner) pullRelation(ctx context.Context, addr string, hash canonical.Digest) error {
	c := &r.rep.Relations
	from, typ, to, _ := splitRelationAddress(addr)
	rm := r.m.Relations[typ]
	row := r.l.get(reconcile.KindRelation, addr)
	if rm == nil { // coverage-ignore: invariant: the diff lists only the types in its scope
		return fmt.Errorf("remotesync: the diff listed %s outside the mapping", addr)
	}
	if strings.Contains(from, entity.StateRefSeparator) {
		c.Rejected++
		r.issue(IssueRejected, addr, "", "the source relation has a tail face; faced rows are not pulled")
		r.record(row, "R", typ, addr, hash, "", stateRejected, nil)
		return nil
	}
	if row != nil && row.State == stateDropped {
		c.Dropped++
		row.Hash = hash
		return nil
	}
	fromRow, toRow := r.byID[from], r.byID[to]
	if (fromRow != nil && fromRow.State == stateDropped) || (toRow != nil && toRow.State == stateDropped) {
		// An endpoint was deleted locally, so the relation is not
		// recreated either.
		c.Dropped++
		r.record(row, "R", typ, addr, hash, localOf(row), stateDropped, nil)
		return nil
	}
	if fromRow == nil || toRow == nil || fromRow.State != stateLinked || toRow.State != stateLinked {
		// An endpoint is not pulled (yet); the relation waits for it.
		c.Deferred++
		return nil
	}
	if r.o.DryRun && (fromRow.Local == "" || toRow.Local == "") {
		// Both endpoints would be created by this run, so the relation would
		// be too.
		c.Created++
		return nil
	}
	key := entity.RelationKey{From: fromRow.Local, FromFace: fromRow.Face, Type: rm.Local, To: toRow.Local}
	var th []theirsValue
	if len(rm.Fields) > 0 {
		if !r.budget(c) {
			return nil
		}
		rel, err := fetchRelation(ctx, r, r.plurals[fromRow.Type], from, typ, to)
		if err != nil {
			rejected, runErr := r.fetchFailed(c, "relation", addr, err)
			if rejected {
				r.record(row, "R", typ, addr, hash, key.String(), stateRejected, nil)
			}
			return runErr
		}
		if th, err = theirs(rm.Fields, rel.Meta, rel.Content, rel.Redacted); err != nil {
			c.Rejected++
			r.issue(IssueRejected, addr, key.String(), err.Error())
			r.record(row, "R", typ, addr, hash, key.String(), stateRejected, nil)
			return nil
		}
	}
	existing, err := r.d.Store.GetRelation(ctx, key)
	switch {
	case errors.Is(err, store.ErrNotFound):
		existing = nil
	case err != nil:
		c.Failed++
		r.issue(IssueFailed, addr, key.String(), err.Error())
		return nil
	}
	if existing == nil && row != nil && row.State == stateLinked && row.Local == key.String() {
		c.Dropped++
		r.record(row, "R", typ, addr, hash, row.Local, stateDropped, nil)
		return nil
	}
	r.writeRelation(ctx, row, addr, typ, hash, key, existing, th)
	return nil
}

// writeRelation creates the local relation, or brings its owned meta to the
// source's values: the source wins on relation meta.
func (r *runner) writeRelation(
	ctx context.Context, row *ledgerRow, addr, typ string, hash canonical.Digest,
	key entity.RelationKey, existing *entity.Relation, th []theirsValue,
) {
	c := &r.rep.Relations
	opts := relationChanges(existing, th)
	changed := len(opts.Properties) > 0 || len(opts.MetaUnset) > 0 || opts.Content != nil
	linked := row != nil && row.State == stateLinked
	adopted := existing != nil && (row == nil || row.Local != key.String() || row.Adopted)
	if existing == nil || changed {
		if err := r.checkRelationMeta(key.Type, existing, opts); err != nil {
			c.Rejected++
			r.issue(IssueRejected, addr, key.String(), err.Error())
			r.record(row, "R", typ, addr, hash, key.String(), stateRejected, nil)
			return
		}
	}
	var err error
	switch {
	case existing == nil && r.o.DryRun:
		c.Created++
		return
	case existing == nil:
		_, err = r.d.Writer.CreateRelation(ctx, key, opts)
		if errors.Is(err, entitymanager.ErrRelationAlreadyExists) {
			c.Deferred++
			r.hold()
			return
		}
		if err == nil {
			c.Created++
		}
	case !changed && linked:
		c.Unchanged++
	case !changed:
		c.Adopted++
	case r.o.DryRun:
		c.Updated++
		return
	default:
		_, err = r.d.Writer.UpdateRelation(ctx, key, opts)
		if err == nil {
			c.Updated++
		}
	}
	if err != nil {
		r.writeFailed(c, addr, key.String(), err)
		if rejectedWrite(err) {
			r.record(row, "R", typ, addr, hash, key.String(), stateRejected, nil)
		}
		return
	}
	r.record(row, "R", typ, addr, hash, key.String(), stateLinked, nil).adopt("", adopted)
}

// relationChanges returns the writes that bring existing's owned fields to
// the source's values; with no existing relation, every reported field.
func relationChanges(existing *entity.Relation, th []theirsValue) entity.RelationOptions {
	opts := entity.RelationOptions{Properties: map[string]any{}}
	for _, t := range th {
		f := t.f.Local
		if existing != nil {
			cur := existing.Properties[f.Name]
			if f.Kind == syncmerge.KindContent {
				cur = existing.Content
			}
			if syncmerge.Equal(f, cur, t.v) {
				continue
			}
		}
		switch {
		case f.Kind == syncmerge.KindContent:
			s, _ := t.v.(string)
			opts.Content = &s
		case t.v == nil:
			if existing != nil {
				opts.MetaUnset = append(opts.MetaUnset, f.Name)
			}
		default:
			opts.Properties[f.Name] = t.v
		}
	}
	return opts
}

// checkRelationMeta checks the meta a relation write leaves against the
// local relation type. The manager's relation writes check the endpoints,
// not the meta, and the meta comes from the source.
func (r *runner) checkRelationMeta(typ string, existing *entity.Relation, opts entity.RelationOptions) error {
	meta := map[string]any{}
	if existing != nil {
		maps.Copy(meta, existing.Properties)
	}
	maps.Copy(meta, opts.Properties)
	for _, name := range opts.MetaUnset {
		delete(meta, name)
	}
	errs := metamodel.NewValidator(r.d.Meta).ValidateRelationProperties(typ, meta)
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("the relation meta does not fit the local schema: %s", strings.Join(msgs, "; "))
}

func (r *runner) goneRelation(ctx context.Context, row *ledgerRow) {
	c := &r.rep.Relations
	if r.m.Remote.onGone() == OnGoneDetach {
		c.Detached++
		r.detach(row)
		return
	}
	if row.Adopted {
		c.Detached++
		r.issue(IssueKept, row.Address, row.Local, "kept and detached: it was there before the pull")
		r.detach(row)
		return
	}
	fromKey, typ, to, ok := splitRelationAddress(row.Local)
	from, err := entity.ParseRef(fromKey)
	if !ok || err != nil {
		c.Failed++
		r.issue(IssueFailed, row.Address, row.Local, "the ledger holds an invalid local relation key")
		return
	}
	if r.o.DryRun {
		c.Deleted++
		return
	}
	key := entity.RelationKey{From: from.ID, FromFace: from.Face, Type: typ, To: to}
	if err := r.d.Writer.DeleteRelation(ctx, key); err != nil && !errors.Is(err, store.ErrNotFound) {
		c.Failed++
		r.issue(IssueFailed, row.Address, row.Local, err.Error())
		return
	}
	c.Deleted++
	r.l.deletes = append(r.l.deletes, r.d.Now().Unix())
	r.d.Logger.Info("remote pull deleted a relation the source no longer lists",
		"remote", r.m.Name, "run_id", r.d.RunID, "source", row.Address, "local", row.Local)
	r.l.remove(row)
}

func (r *runner) goneEntity(ctx context.Context, row *ledgerRow) {
	c := &r.rep.Entities
	if r.m.Remote.onGone() == OnGoneDetach {
		c.Detached++
		r.detach(row)
		return
	}
	ours, err := r.d.Store.GetEntity(ctx, row.localRef())
	switch {
	case errors.Is(err, store.ErrNotFound):
		if !r.o.DryRun {
			r.l.remove(row)
		}
		return
	case err != nil:
		c.Failed++
		r.issue(IssueFailed, row.Address, row.Local, err.Error())
		return
	}
	if row.Adopted {
		c.Detached++
		r.issue(IssueKept, row.Address, row.Local, "kept and detached: it was there before the pull")
		r.detach(row)
		return
	}
	if field, edited := r.editedLocally(row, ours); edited {
		c.Detached++
		r.issue(IssueKept, row.Address, row.Local, "kept and detached: owned field "+field+" was edited locally")
		r.detach(row)
		return
	}
	if r.o.DryRun {
		c.Deleted++
		return
	}
	_, err = r.d.Writer.DeleteEntity(ctx, row.Local, false)
	switch {
	case errors.Is(err, entitymanager.ErrHasRelations):
		c.Detached++
		r.issue(IssueKept, row.Address, row.Local, "kept and detached: it has local relations")
		r.detach(row)
		return
	case err != nil && !errors.Is(err, store.ErrNotFound):
		c.Failed++
		r.issue(IssueFailed, row.Address, row.Local, err.Error())
		return
	}
	c.Deleted++
	r.l.deletes = append(r.l.deletes, r.d.Now().Unix())
	r.d.Logger.Info("remote pull deleted an entity the source no longer lists",
		"remote", r.m.Name, "run_id", r.d.RunID, "source", row.Address, "local", row.Local)
	r.l.remove(row)
}

// editedLocally reports an owned field whose local value is not the base.
func (r *runner) editedLocally(row *ledgerRow, ours *entity.Entity) (string, bool) {
	typ, _, _ := strings.Cut(row.Address, "/")
	em := r.m.Entities[typ]
	if em == nil {
		return "", true
	}
	for _, f := range em.Fields {
		b, ok := row.Base[f.baseKey()]
		if !ok || syncmerge.Digest(f.Local, oursValue(f.Local, ours)) != b {
			return f.Local.Name, true
		}
	}
	return "", false
}

// detach marks row detached, outside a dry run.
func (r *runner) detach(row *ledgerRow) {
	if !r.o.DryRun {
		row.State = stateDetached
	}
}
