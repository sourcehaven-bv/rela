package remotesync

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/reconclient"
	"github.com/Sourcehaven-BV/rela/internal/reconclient/httpremote"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/syncmerge"
)

// Writer is the write path a run uses. The entity manager implements
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
	store.EntityLister
	GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error)
	GetRelation(ctx context.Context, k entity.RelationKey) (*entity.Relation, error)
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// ReadGate reads a local row as the principal on ctx may see it. A row
// the principal cannot read is [store.ErrNotFound], as an absent one is.
// The run asks it before it names a local row it found through the
// [RawReader], so a report never identifies a row its reader cannot see.
// A report still says that a ref or an endpoint is held by a row the
// puller cannot read. That one bit is accepted: a create on the ref would
// be refused by the unique check anyway. An implementation may answer a
// gate failure as ErrNotFound, as the visibility reader does; any other
// error fails the row.
type ReadGate interface {
	GetAddress(ctx context.Context, addr string) (*entity.Entity, error)
}

// Fingerprinter returns how many rows a range holds and the fingerprint of
// their addresses and view hashes in its wire form. relations says whether
// the rows are relations or entities.
type Fingerprinter func(relations bool, rows iter.Seq2[string, canonical.Digest]) (count uint64, fingerprint string)

// Deps are the collaborators of one run.
type Deps struct {
	// Source is the remote.
	Source Source
	// Pusher sends the reads and writes of a remote with push; required
	// for one. The wiring site passes the same remote as Source.
	Pusher Pusher
	Meta   *metamodel.Metamodel
	Writer Writer
	Store  RawReader
	// Gate reads local rows as the principal on ctx may see them.
	Gate ReadGate
	// State holds the ledger.
	State state.KV
	// Fingerprint sums ledger rows exactly as the source sums its own
	// range, so a diff compares like with like. The wiring site supplies
	// the visibility package's.
	Fingerprint Fingerprinter
	Logger      *slog.Logger
	// RunID names the run in the log lines of its deletes.
	RunID string
	// DiffDeadline bounds the diff; zero means the reconcile default.
	DiffDeadline time.Duration
	// Now is the clock of the gone guard's delete window; nil means
	// time.Now.
	Now func() time.Time
	// RelationMetaGated reports whether a local relation type's meta has
	// `visible:` grants, so a reader may see less of it than is stored. On
	// a remote with push such a type is pulled source-wins and not pushed,
	// because the run cannot tell which of its meta the principal may read.
	// Nil counts every type as gated.
	RelationMetaGated func(relType string) bool
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
	case d.Gate == nil:
		return errors.New("remotesync: a read gate is required")
	case d.State == nil:
		return errors.New("remotesync: a state store is required")
	case d.Fingerprint == nil:
		return errors.New("remotesync: a fingerprint function is required")
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

// ErrGoneGuard reports a run whose deletes and detaches the gone guard
// held. The rest of the run was applied.
var ErrGoneGuard = errors.New("remotesync: refused by the gone guard")

// Dial returns the client of remote p, authenticated with credential, the
// value [Remote.ReadCredential] returned. transport may be nil. The client
// is the run's [Source], and its [Pusher] on a remote with push.
func Dial(p Remote, credential string, transport http.RoundTripper) (*httpremote.Remote, error) {
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
// variable; secrets loads the global section of the project's secrets.
// Errors name the source, never the value.
//
// Nil: getenv and secrets are accepted; the source that needs a nil one
// fails.
func (p Remote) ReadCredential(getenv func(string) string, secrets func() (map[string]string, error)) (string, error) {
	if p.Credential.IsZero() {
		return "", errors.New("remotesync: credential: no source")
	}
	v, err := p.Credential.ReadDeclared(getenv, httpremote.GlobalSecrets(secrets))
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
	if m.Remote.AllowInsecureHTTP && strings.HasPrefix(strings.ToLower(m.Remote.URL), "http:") {
		d.Logger.Warn("remote pull sends its credential over plain http (allow_insecure_http)",
			"remote", m.Name, "run_id", d.RunID)
	}
	if m.Remote.Push && d.Pusher == nil {
		return rep, errors.New("remotesync: a remote with push needs a pusher")
	}
	l, err := loadLedger(ctx, d.State, m)
	if err != nil {
		return rep, err
	}
	held, err := loadConflicts(ctx, d.State, m.Name)
	if err != nil {
		return rep, err
	}
	r := &runner{m: m, d: d, o: o, l: l, c: held, rep: &rep}
	if sweepErr := r.sweep(ctx); sweepErr != nil {
		return rep, sweepErr
	}
	if prepErr := r.prepare(ctx); prepErr != nil {
		return rep, prepErr
	}
	diff, err := r.diff(ctx)
	if err != nil {
		return rep, err
	}
	gone, sends, guardErr := guarder{r}.guard(diff)
	runErr := r.apply(ctx, diff, gone)
	if runErr == nil && m.Remote.Push {
		rep.Pushed = &PushCounts{}
		runErr = pusher{r}.push(ctx)
		if runErr == nil {
			rep.PushedRelations = &PushCounts{}
			runErr = relSync{r}.push(ctx)
		}
		if runErr == nil {
			runErr = deleter{r}.send(ctx, sends)
		}
	}
	runErr = r.finish(ctx, runErr)
	return rep, errors.Join(runErr, guardErr)
}

// runner holds the state of one run.
type runner struct {
	m   *Mapping
	d   Deps
	o   Options
	l   *ledger
	c   *conflicts
	rep *Report

	plurals map[string]string
	// byID maps a source entity id to its ledger row. Source ids are
	// unique across types, as rela ids are.
	byID map[string]*ledgerRow
	// goneRows are the linked rows the source no longer lists, whether or
	// not the guard lets the run act on them.
	goneRows map[*ledgerRow]bool
	// fetched counts the rows fetched against max_fetch, and read the
	// bytes they took against max_bytes.
	fetched int
	read    int64
	// cur is the source row being pulled. An issue raised while it is set
	// holds the row, so later runs take it last (see [ledgerDoc.Held]).
	cur *heldRow
	// pushSeen holds the local ids the push read, or nil when no push ran.
	pushSeen map[string]bool
	// localTimes holds the last-updated time of each local row when the run
	// first read it, before its own writes.
	localTimes map[string]time.Time
	// relSeen holds the local relation keys the relation push read, or nil
	// when it did not finish.
	relSeen map[string]bool
}

// finish counts the conflict list, saves the ledger and the list, and turns
// failed rows into the run's error when nothing else failed.
func (r *runner) finish(ctx context.Context, runErr error) error {
	rep := r.rep
	r.forgetConflicts()
	for _, e := range r.c.entries {
		if e.Reason == ReasonSuperseded {
			rep.Superseded++
		} else {
			rep.Held++
		}
	}
	if !r.o.DryRun {
		wctx := context.WithoutCancel(ctx)
		if err := r.l.save(wctx, r.d.State, r.m); err != nil {
			return errors.Join(runErr, err)
		}
		if err := r.c.save(wctx, r.d.State); err != nil {
			return errors.Join(runErr, err)
		}
	}
	failed := rep.Entities.Failed + rep.Relations.Failed
	if rep.Pushed != nil {
		failed += rep.Pushed.Failed
	}
	if rep.PushedRelations != nil {
		failed += rep.PushedRelations.Failed
	}
	if runErr == nil && failed > 0 {
		runErr = fmt.Errorf("remotesync: %d rows failed", failed)
	}
	return runErr
}

// heldRow names a source row at one source hash.
type heldRow struct {
	key  string
	hash canonical.Digest
}

// sweep turns the ledger row of every linked or detached local row that is
// gone into a dropped row, and clears its field digests. So no digest of a
// deleted row's values outlives it, and the pull still does not recreate
// it. On a remote with send_deletes a linked row becomes a deleting row
// instead, which keeps its digests until the push has deleted the remote
// row; a deleting row whose local row is back is linked again. It reads the
// ledger's own ids: one header scan per mapped entity type and one relation
// scan per mapped relation type. A row of a type the mapping no longer
// names is left alone.
func (r *runner) sweep(ctx context.Context) error {
	ids := map[string][]string{}
	from := map[string][]string{}
	var rows []*ledgerRow
	for _, row := range r.l.rows {
		if why := r.expiredDelete(row); why != "" {
			r.issue(IssueKept, row.Address, row.Local, why)
			row.State, row.Base, row.Resolve, row.Gone = stateDropped, nil, "", time.Time{}
			continue
		}
		if row.Local == "" || (row.State != stateLinked && row.State != stateDetached && row.State != stateDeleting) {
			continue
		}
		if em, ok := r.m.Entities[row.Type]; ok && row.Kind == "E" {
			ids[em.Local] = append(ids[em.Local], row.Local)
		} else if rm, ok := r.m.Relations[row.Type]; ok && row.Kind == "R" {
			fromKey, _, _, ok := splitRelationAddress(row.Local)
			ref, err := entity.ParseRef(fromKey)
			if !ok || err != nil {
				continue
			}
			from[rm.Local] = append(from[rm.Local], ref.ID)
		} else {
			continue
		}
		rows = append(rows, row)
	}
	present, err := r.present(ctx, ids, from)
	if err != nil {
		return err
	}
	for _, row := range rows {
		switch {
		case row.State == stateDeleting && present[row.Kind+row.Local]:
			row.State, row.Resolve, row.Gone = stateLinked, "", time.Time{}
		case row.State == stateDeleting || present[row.Kind+row.Local]:
		case row.State == stateLinked && r.m.Remote.SendDeletes:
			row.State, row.Gone = stateDeleting, r.d.Now()
		default:
			row.State, row.Base, row.Resolve = stateDropped, nil, ""
		}
	}
	return nil
}

// present lists which of the ledger's local rows the store still holds:
// entities by type and id, relations by type and source id. Keys are the
// row kind plus its local address.
func (r *runner) present(ctx context.Context, ids, from map[string][]string) (map[string]bool, error) {
	present := map[string]bool{}
	for _, typ := range sortedKeys(ids) {
		q := store.EntityQuery{Type: typ, IDs: ids[typ], Faces: store.AllFaces()}
		for h, err := range store.ListEntityHeaders(ctx, r.d.Store, q) {
			if err != nil {
				return nil, fmt.Errorf("remotesync: list local %s rows: %w", typ, err)
			}
			present["E"+h.ID] = true
		}
	}
	for _, typ := range sortedKeys(from) {
		q := store.RelationQuery{Type: typ, EntityIDs: from[typ], Direction: store.DirectionOutgoing}
		for rel, err := range r.d.Store.ListRelations(ctx, q) {
			if err != nil {
				return nil, fmt.Errorf("remotesync: list local %s relations: %w", typ, err)
			}
			present["R"+rel.Key()] = true
		}
	}
	return present, nil
}

// expiredDelete reports why a deleting row is no longer to be deleted on
// the remote: send_deletes was turned off, or the row waited longer than
// [deleteRetention]. It returns "" for any other row.
func (r *runner) expiredDelete(row *ledgerRow) string {
	switch {
	case row.State != stateDeleting:
		return ""
	case !r.m.Remote.SendDeletes:
		return "not deleted on the remote: send_deletes is off"
	case !row.Gone.IsZero() && r.d.Now().Sub(row.Gone) > deleteRetention:
		return "not deleted on the remote: the delete waited longer than 30 days"
	}
	return ""
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
	r.resolved()
	r.byID = map[string]*ledgerRow{}
	for _, row := range r.l.rows {
		if row.Kind == "E" {
			_, id, _ := strings.Cut(row.Address, "/")
			r.byID[id] = row
		}
	}
	return nil
}

// resolved acts on the delete conflicts a person resolved since the last
// run. A deleting row resolved to the remote side is forgotten, so the pull
// restores it as a new local row. A detached row resolved to the remote side
// is linked again, so the gone path deletes the local row without the edit
// check. The other two sides are acted on later in the run: the delete is
// sent without the edit check, and the push restores the row on the remote.
// A dry run changes nothing.
func (r *runner) resolved() {
	if r.o.DryRun {
		return
	}
	for _, row := range r.l.rows {
		switch {
		case row.Resolve != TakeRemote:
		case row.State == stateDeleting:
			r.l.remove(row)
		case row.State == stateDetached:
			row.State = stateLinked
		}
	}
}

func (r *runner) diff(ctx context.Context) (reconclient.Result, error) {
	r.l.buildIndex()
	rows := r.m.Remote.maxRows()
	differ, err := reconclient.New(local{l: r.l, sum: r.d.Fingerprint}, r.d.Source, reconclient.Options{
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

// apply pulls the new and changed rows, entities first so relations find
// their endpoints, then handles the gone rows, relations first so an
// entity's pulled relations are gone before it is.
func (r *runner) apply(ctx context.Context, diff reconclient.Result, gone []*ledgerRow) error {
	ents, rels := r.work(diff)
	pull := func(kind reconclient.Kind, it workItem, f func(context.Context, string, canonical.Digest) error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.cur = &heldRow{key: rowKey(kind, it.addr), hash: it.hash}
		defer func() { r.cur = nil }()
		return f(ctx, it.addr, it.hash)
	}
	for _, it := range ents {
		if err := pull(reconclient.KindEntity, it, r.pullEntity); err != nil {
			return err
		}
	}
	for _, it := range rels {
		if err := pull(reconclient.KindRelation, it, r.pullRelation); err != nil {
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
				goner{r}.goneRelation(ctx, row)
			default:
				goner{r}.goneEntity(ctx, row)
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
	order := func(kind reconclient.Kind, items []workItem) {
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
	order(reconclient.KindEntity, ents)
	order(reconclient.KindRelation, rels)
	for k := range r.l.held {
		if !listed[k] {
			delete(r.l.held, k)
		}
	}
	return ents, rels
}

// kindLetter is the ledger's letter for k: "E" or "R".
func kindLetter(k reconclient.Kind) string {
	if k == reconclient.KindRelation {
		return "R"
	}
	return "E"
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
		if f.toContent() {
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
	conflicts []fieldConflict
	// settled are the fields both sides changed that a rule decided.
	settled []fieldConflict
	// agreed are the reported fields that are not held: those that agree
	// or merged, and the settled ones.
	agreed []FieldMapping
	// verdict is the latest rule's judgement of the row.
	verdict verdict
}

// fieldConflict is one field both sides changed: both values and their
// digests. winner is the side a rule picked, or "" for a held field, with
// fallback saying why a latest rule held it.
type fieldConflict struct {
	f                FieldMapping
	theirs, oursV    any
	ours, theirsD    string
	winner, fallback string
}

// conflictNames returns the local names of the fields in conflict.
func (m merged) conflictNames() []string {
	out := make([]string, len(m.conflicts))
	for i, c := range m.conflicts {
		out[i] = c.f.Local.Name
	}
	return out
}

func (m merged) empty() bool { return len(m.props) == 0 && len(m.unset) == 0 && m.content == nil }

// merge compares each reported owned field on digests: the same value on
// both sides is unchanged; a local value still at the base takes theirs; a
// source value still at the base keeps the local value; anything else, and
// any difference without a base, is a conflict. The field's rule decides a
// conflict, with v as the latest rule's judgement of the row. A decided
// field moves its base to the source value: the remote's value is then
// written locally, and the local one is pushed and stays put on a pull. A
// held field keeps its base and its local value; the other fields still
// merge.
func merge(fields []FieldMapping, base map[string]string, ours *entity.Entity, th []theirsValue, v verdict) merged {
	out := merged{props: map[string]any{}, base: map[string]string{}, verdict: v}
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
		case hasBase && b == do:
			out.set(f, t.v)
			out.base[k] = dt
		case hasBase && b == dt:
			// The local edit stands; the base stays at the source value.
		default:
			c := fieldConflict{f: t.f, theirs: t.v, oursV: oursValue(f, ours), ours: do, theirsD: dt}
			c.winner, c.fallback = decide(t.f.Rule, v)
			if c.winner == "" {
				out.conflicts = append(out.conflicts, c)
				continue
			}
			if c.winner == TakeRemote {
				out.set(f, t.v)
			}
			out.base[k] = dt
			out.settled = append(out.settled, c)
		}
		out.agreed = append(out.agreed, t.f)
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
	row := r.l.get(reconclient.KindEntity, addr)
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
	if row != nil && row.State == stateDeleting {
		// The push deletes it; it compares the remote row itself.
		return nil
	}
	if !r.budget(c) {
		return nil
	}
	e, etag, err := r.fetch(ctx, typ, id)
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
	r.mergeFetched(ctx, em, row, addr, id, hash, etag, sourceRow{th: th, updated: parseStamp(e.Updated)})
	return nil
}

// sourceRow is a fetched source row as a merge reads it: its owned fields
// and its last-updated time.
type sourceRow struct {
	th      []theirsValue
	updated stamp
}

// fetch reads one source entity, with its ETag on a remote with push.
func (r *runner) fetch(ctx context.Context, typ, id string) (remoteEntity, string, error) {
	if r.d.Pusher == nil {
		e, err := fetchEntity(ctx, r, r.plurals[typ], typ, id)
		return e, "", err
	}
	return fetchTagged(ctx, r, r.plurals[typ], typ, id)
}

// Send sends through the pusher and counts the bytes against max_bytes.
func (r *runner) Send(
	ctx context.Context, method, path string, body []byte, ifMatch string, limit int64,
) (httpremote.Answer, error) {
	ans, err := r.d.Pusher.Send(ctx, method, path, body, ifMatch, limit)
	r.read += int64(len(ans.Body))
	return ans, err
}

// mergeFetched merges one fetched source row into its local row, trying
// again when the local row changes under the write, and keeps the row's
// ETag for the next push.
func (r *runner) mergeFetched(
	ctx context.Context, em *EntityMapping, row *ledgerRow, addr, id string, hash canonical.Digest, etag string,
	src sourceRow,
) {
	for range maxAttempts {
		if !r.applyEntity(ctx, em, row, addr, id, hash, src) {
			if cur := r.l.get(reconclient.KindEntity, addr); cur != nil && cur.State == stateLinked && !r.o.DryRun {
				cur.ETag = etag
			}
			return
		}
		row = r.l.get(reconclient.KindEntity, addr)
	}
	r.rep.Entities.Failed++
	r.issue(IssueFailed, addr, localOf(row), "the local row kept changing during the run")
}

// applyEntity merges one fetched source row into its local row. It returns
// true when the local row changed under the write and the row should be
// tried again. Every other outcome is counted in the report.
func (r *runner) applyEntity(
	ctx context.Context, em *EntityMapping, row *ledgerRow, addr, id string, hash canonical.Digest, src sourceRow,
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
		r.create(ctx, em, row, addr, id, hash, src.th)
		return false
	}
	readable, ourStamp, err := r.localStamp(ctx, ours)
	if err != nil {
		r.gateFailed(c, addr, err)
		return false
	}
	if !readable {
		// The ref is taken, so the row cannot be created either. The issue
		// does not name the row: a hidden row is reported as no row would
		// be, except that the ref is held.
		c.Conflicts++
		r.issue(IssueConflict, addr, "", "the ref is held by a local row the puller cannot read")
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
	var synced time.Time
	if row != nil {
		synced = row.Synced
	}
	v := latest(ourStamp, src.updated, synced, r.d.Now(), r.m.Remote.maxClockSkew())
	mg := merge(em.Fields, baseOf(row), ours, src.th, v)
	r.noteConflicts(conflictTarget{typ: em.Local, local: ours.ID, addr: addr}, mg)
	c.Settled += len(mg.settled)
	if len(mg.conflicts) > 0 {
		c.Conflicts++
		r.issue(IssueConflict, addr, ours.ID, "both sides changed these fields; held on the conflict list",
			mg.conflictNames()...)
	}
	switch {
	case mg.empty() && len(mg.conflicts) > 0:
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
	rec := r.record(row, "E", em.Source, addr, hash, ours.ID, stateLinked, mg.base)
	rec.adopt(ours.Face, adopted)
	r.markSynced(rec, ours.ID)
	return false
}

// localStamp reports whether the principal on ctx may read the local row
// ours, and the row's last-updated time as the run may use it for the
// latest rule. The time is withheld when the principal does not see every
// field of the row, as the API withholds it, and it is the time the row had
// when the run first saw it: a write by this run does not count as a local
// edit.
func (r *runner) localStamp(ctx context.Context, ours *entity.Entity) (bool, stamp, error) {
	gated, err := r.d.Gate.GetAddress(ctx, ours.Ref().String())
	switch {
	case errors.Is(err, store.ErrNotFound):
		return false, stamp{}, nil
	case err != nil:
		return false, stamp{}, fmt.Errorf("read the local row: %w", err)
	}
	at, seen := r.localTimes[ours.ID]
	if !seen {
		at = ours.UpdatedAt
		if r.localTimes == nil {
			r.localTimes = map[string]time.Time{}
		}
		r.localTimes[ours.ID] = at
	}
	if len(gated.Redacted) > 0 || len(gated.Inaccessible) > 0 {
		return true, stamp{}, nil
	}
	return true, stamp{At: at}, nil
}

// markSynced records that both sides agree on every field of row now, unless
// a field of the local row local is still held. A nil row (a dry run) is
// left alone.
func (r *runner) markSynced(row *ledgerRow, local string) {
	if row == nil || r.c.holdsField(local) {
		return
	}
	row.Synced = r.d.Now()
}

// readable reports whether the principal on ctx may read the local row addr
// names, a row the raw reader returned or the ledger holds. A bare id
// resolves in the gate's world.
func (r *runner) readable(ctx context.Context, addr string) (bool, error) {
	_, err := r.d.Gate.GetAddress(ctx, addr)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, store.ErrNotFound):
		return false, nil
	}
	return false, fmt.Errorf("read the local row: %w", err)
}

// gateFailed reports a row whose local row could not be checked with the
// gate. The error may name the row, so it goes to the operator's log and
// not to the report.
func (r *runner) gateFailed(c *Counts, addr string, err error) {
	r.d.Logger.Error("remote pull: check the local row", "run_id", r.d.RunID, "address", addr, "err", err)
	c.Failed++
	r.issue(IssueFailed, addr, "", "could not check whether the puller can read the local row")
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
	// The count includes rows the puller may not read, so it goes to the
	// log only.
	r.d.Logger.Warn("remote pull: more than one local entity holds a ref",
		"run_id", r.d.RunID, "system", r.m.System, "ref", id, "count", len(found))
	return nil, fmt.Errorf("more than one local entity holds ref %q of system %q", id, r.m.System)
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
	rec := r.record(row, "E", em.Source, addr, hash, res.Entity.ID, stateLinked, base)
	rec.adopt(res.Entity.Face, false)
	r.markSynced(rec, res.Entity.ID)
}

// uniqueOn reports a unique violation on prop. A violation that names no
// property is taken to be on prop, the external ref the create sets.
func uniqueOn(err error, prop string) bool {
	if u, ok := errors.AsType[store.UniquePropertyError](err); ok {
		return u.Property == "" || u.Property == prop
	}
	for _, e := range schemaErrors(err) {
		if e.Type == metamodel.ValidationErrorUnique && (e.Property == "" || e.Property == prop) {
			return true
		}
	}
	return false
}

// rejectedWrite reports a write the local schema refused: a validation
// error, a unique value, a relation the schema does not allow between the
// endpoints, or a relation over its cardinality. It is not tried again
// until the source row changes. The writer's errors wrap the metamodel's
// own errors and the [entity] sentinels, so they are matched without
// knowing the writer.
func rejectedWrite(err error) bool {
	_, badType := errors.AsType[*metamodel.InvalidRelationError](err)
	_, noType := errors.AsType[*metamodel.RelationNotFoundError](err)
	_, unique := errors.AsType[store.UniquePropertyError](err)
	return len(schemaErrors(err)) > 0 || badType || noType || unique ||
		errors.Is(err, entity.ErrCardinalityExceeded)
}

// schemaErrors returns every metamodel validation error err wraps.
func schemaErrors(err error) []*metamodel.ValidationError {
	var out []*metamodel.ValidationError
	var walk func(error)
	walk = func(e error) {
		switch x := e.(type) { //nolint:errorlint // the walk visits every wrapped error itself, to collect them all
		case nil:
		case *metamodel.ValidationError:
			out = append(out, x)
		case interface{ Unwrap() []error }:
			for _, c := range x.Unwrap() {
				walk(c)
			}
		case interface{ Unwrap() error }:
			walk(x.Unwrap())
		}
	}
	walk(err)
	return out
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

// goneHere reports whether an entity row's local row was deleted here.
func goneHere(row *ledgerRow) bool {
	return row != nil && (row.State == stateDropped || row.State == stateDeleting)
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
	row := r.l.get(reconclient.KindRelation, addr)
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
	if row != nil && row.State == stateDeleting {
		return nil
	}
	fromRow, toRow := r.byID[from], r.byID[to]
	if goneHere(fromRow) || goneHere(toRow) {
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
	if !r.endpointsReadable(ctx, c, addr, fromRow, toRow) {
		return nil
	}
	key := entity.RelationKey{From: fromRow.Local, FromFace: fromRow.Face, Type: rm.Local, To: toRow.Local}
	src, ok, err := relSync{r}.fetchSource(ctx, row, rm, addr, hash, key, r.plurals[fromRow.Type])
	if !ok {
		return err
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
	if existing != nil && (relSync{r}).merges(rm) {
		relSync{r}.mergeRelation(ctx, rm, row, addr, hash, key, existing, src)
		return nil
	}
	r.writeRelation(ctx, row, addr, typ, hash, key, existing, src.th)
	return nil
}

// endpointsReadable reports whether the puller may read both local
// endpoints of a relation. When it may not, the relation is reported as a
// conflict that names neither endpoint.
func (r *runner) endpointsReadable(ctx context.Context, c *Counts, addr string, ends ...*ledgerRow) bool {
	for _, end := range ends {
		if end.Local == "" { // a dry run creates it
			continue
		}
		readable, err := r.readable(ctx, end.localRef().String())
		if err != nil {
			r.gateFailed(c, addr, err)
			return false
		}
		if !readable {
			c.Conflicts++
			r.issue(IssueConflict, addr, "", "an endpoint is a local row the puller cannot read")
			return false
		}
	}
	return true
}

// writeRelation creates the local relation, or brings its owned meta to the
// source's values: the source wins on relation meta, except where
// [relSync.mergeRelation] merges it. The ledger row keeps the source's meta
// as the merge base.
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
		if errors.Is(err, entity.ErrRelationAlreadyExists) {
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
	rec := r.record(row, "R", typ, addr, hash, key.String(), stateLinked, metaBase(th))
	rec.adopt("", adopted)
	r.markSynced(rec, key.String())
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

// detach marks row detached, outside a dry run. A side a person chose for
// it is spent.
func (r *runner) detach(row *ledgerRow) {
	if !r.o.DryRun {
		row.State, row.Resolve = stateDetached, ""
	}
}
