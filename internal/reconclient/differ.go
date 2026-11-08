package reconclient

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/reconwire"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Local is the local side: the gated element range read for the principal
// bound on ctx. [Visible] adapts [visibility.Elements] to it.
//
// Sum returns [visibility.ErrRangeTooLarge] when the range is too large to
// fingerprint, and [visibility.ErrRelationMetaGated] when the policy can
// hide meta of a relation type, as [visibility.Elements.Fingerprint] does.
type Local interface {
	Sum(ctx context.Context, types visibility.ElementTypes, r store.ElementRange) (Sum, error)
	List(ctx context.Context, types visibility.ElementTypes, r store.ElementRange, limit int) (Page, error)
}

// Kind says whether an element is an entity or a relation. The zero Kind is
// invalid.
type Kind byte

// The two kinds, entities first.
const (
	KindEntity Kind = iota + 1
	KindRelation
)

// String returns "entity" or "relation", the kind's name on the wire.
func (k Kind) String() string {
	if k == KindRelation {
		return "relation"
	}
	return "entity"
}

// Element is one row as one side sees it.
type Element struct {
	Kind Kind
	// Address is "type/id[@face]" for an entity and "from[@face]--type--to"
	// for a relation.
	Address string
	// Hash is the view hash: the canonical digest of the reader's view.
	Hash canonical.Digest
}

// Sum is the fingerprint of a range: how many elements it holds, and the
// sum of their fingerprints as 32 lower-case hex digits, the wire form.
// Two sums are compared with ==.
type Sum struct {
	Count       uint64
	Fingerprint string
}

// Row is one element of a [Page]; its kind is the kind of the range listed.
type Row struct {
	Address string
	Hash    canonical.Digest
}

// Page is one page of a local listing, with the meaning of
// [visibility.ElementPage]: the caller resumes after Last.
type Page struct {
	Rows []Row
	Last string
	More bool
}

// Remote sends one request to POST /api/v1/_reconcile. It returns
// [*RefusedError] for an admission refusal and [*RangeError] for a refused
// range; the answer itself is validated by the Differ.
type Remote interface {
	Reconcile(ctx context.Context, req reconwire.Request) (reconwire.Response, error)
}

// The server's request limits (reconcile_handler.go). A request over them is
// refused as a whole, so the client packs within them.
const (
	serverMaxRanges    = 32
	serverMaxTypeNames = 256
	serverMaxListRows  = visibility.DefaultElementMaxRows
)

// Tuning of the recursion.
const (
	// listThreshold: a differing span with at most this many local elements
	// is listed rather than split. Splitting sends local addresses to the
	// remote as range bounds, so a small span is never split.
	listThreshold = 256
	// fanout is the number of buckets a differing span is split into.
	fanout = 16
	// bucketRows bounds a bucket after a too_large answer, under the
	// server's row cap, so the next fingerprint fits.
	bucketRows = 1500
	// remotePage is the largest list page asked of the remote: the server's
	// row cap, so a listing alone in a request uses all of it.
	remotePage = serverMaxListRows
	// pageSlack is added to the expected count of a small listing, so rows
	// written since the fingerprint still fit one page.
	pageSlack = 16
	// localPage is the page size of a local listing.
	localPage = 1000
)

// Options bounds a run. The zero value takes every default.
type Options struct {
	// MaxRoundTrips caps the requests sent, retries included. Default 1000.
	MaxRoundTrips int
	// MaxRemoteElements caps the elements listed from the remote. Default
	// 1,000,000.
	MaxRemoteElements int
	// MaxLocalElements caps the elements listed locally. Default 1,000,000.
	MaxLocalElements int
	// MaxRemoteBytes caps the address bytes kept from the remote. Default
	// 256 MiB.
	MaxRemoteBytes int
	// Deadline bounds the whole run. Default 10 minutes.
	Deadline time.Duration
	// MaxRetryWait caps one wait for a refused request. Default 60 seconds.
	MaxRetryWait time.Duration
	// MaxRefusals caps the refusals of one request in a row. Default 8.
	MaxRefusals int
	// MaxNotRun caps the not_run answers for one span. Default 4.
	MaxNotRun int
	// Sleep waits d or until ctx ends.
	//
	// Nil: accepted, meaning a timer. Tests inject their own.
	Sleep func(ctx context.Context, d time.Duration) error
}

// The defaults of [Options]. With full pages, defaultMaxRoundTrips covers
// defaultMaxElements remote elements twice over.
const (
	defaultMaxRoundTrips = 1000
	defaultMaxElements   = 1_000_000
	defaultMaxBytes      = 256 << 20
	defaultMaxRefusals   = 8
	defaultMaxNotRun     = 4
	defaultDeadline      = 10 * time.Minute
	defaultMaxRetryWait  = time.Minute
)

func (o Options) withDefaults() Options {
	def := func(v *int, d int) {
		if *v <= 0 {
			*v = d
		}
	}
	def(&o.MaxRoundTrips, defaultMaxRoundTrips)
	def(&o.MaxRemoteElements, defaultMaxElements)
	def(&o.MaxLocalElements, defaultMaxElements)
	def(&o.MaxRemoteBytes, defaultMaxBytes)
	def(&o.MaxRefusals, defaultMaxRefusals)
	def(&o.MaxNotRun, defaultMaxNotRun)
	if o.Deadline <= 0 {
		o.Deadline = defaultDeadline
	}
	if o.MaxRetryWait <= 0 {
		o.MaxRetryWait = defaultMaxRetryWait
	}
	if o.Sleep == nil {
		o.Sleep = sleep
	}
	return o
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Scope names the types to compare. A relation is compared only when both
// of its endpoints are rows of Entities that the principal may read, on each
// side; every relation range sends all of Entities as its endpoint scope.
type Scope struct {
	Entities  []string
	Relations []string
}

// Change is an address on both sides with different view hashes.
type Change struct {
	Kind    Kind
	Address string
	Local   canonical.Digest
	Remote  canonical.Digest
}

// Skipped is a relation type left out of the comparison.
type Skipped struct {
	Kind Kind
	Type string
	// Remote reports that the remote's policy refused the type; otherwise
	// the local policy did.
	Remote bool
	Reason string
}

// Stats counts the work of a run.
type Stats struct {
	RoundTrips     int
	LocalElements  int
	RemoteElements int
}

// Result is the outcome of [Differ.Diff]. Every list is ordered by kind,
// then bytewise by address.
type Result struct {
	OnlyLocal  []Element
	OnlyRemote []Element
	Differing  []Change
	Skipped    []Skipped
	Stats      Stats
}

// Same reports whether no difference was found. Skipped types are not
// differences.
func (r Result) Same() bool {
	return len(r.OnlyLocal) == 0 && len(r.OnlyRemote) == 0 && len(r.Differing) == 0
}

// Differ compares a local and a remote element set.
type Differ struct {
	local  Local
	remote Remote
	opt    Options
}

// New returns a Differ.
//
// Nil: local and remote rejected; there is nothing to compare without them.
func New(local Local, remote Remote, opt Options) (*Differ, error) {
	switch {
	case local == nil:
		return nil, errors.New("reconclient: local element reader is required")
	case remote == nil:
		return nil, errors.New("reconclient: remote is required")
	}
	return &Differ{local: local, remote: remote, opt: opt.withDefaults()}, nil
}

// Diff compares the elements of scope on both sides.
//
// Both sides may change while it runs; the result then mixes moments. A row
// written during the run may show up as a difference, or be missed until the
// next run.
func (d *Differ) Diff(ctx context.Context, scope Scope) (Result, error) {
	spans, err := initialSpans(scope)
	if err != nil {
		return Result{}, err
	}
	runCtx, cancel := context.WithTimeout(ctx, d.opt.Deadline)
	defer cancel()
	r := &run{d: d, ctx: runCtx}
	work := make([]*task, 0, len(spans))
	for _, s := range spans {
		work = append(work, &task{sp: s})
	}
	for len(work) > 0 {
		if work, err = r.round(work); err != nil {
			if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
				return Result{}, fmt.Errorf("%w: the run took longer than %s: %w", ErrBudget, d.opt.Deadline, err)
			}
			return Result{}, err
		}
	}
	return r.result(), nil
}

// initialSpans checks scope and returns the spans a run starts with: the
// entity types and the relation types, each chunked so one range stays under
// the server's type name cap.
func initialSpans(scope Scope) ([]*span, error) {
	for _, names := range [][]string{scope.Entities, scope.Relations} {
		for i, n := range names {
			if n == "" {
				return nil, errors.New("reconclient: empty type name")
			}
			if slices.Contains(names[:i], n) {
				return nil, fmt.Errorf("reconclient: type %q is named twice", n)
			}
		}
	}
	if len(scope.Relations) > 0 && len(scope.Entities) == 0 {
		return nil, errors.New("reconclient: relation types need the entity types of their endpoints in scope")
	}
	if len(scope.Relations) > 0 && len(scope.Entities) >= serverMaxTypeNames {
		return nil, fmt.Errorf("reconclient: relations need fewer than %d entity types in scope", serverMaxTypeNames)
	}
	ents := slices.Sorted(slices.Values(scope.Entities))
	var out []*span
	for _, c := range chunk(ents, serverMaxTypeNames) {
		out = append(out, &span{kind: KindEntity, entities: c})
	}
	if len(scope.Relations) == 0 {
		return out, nil
	}
	rels := slices.Sorted(slices.Values(scope.Relations))
	for _, c := range chunk(rels, serverMaxTypeNames-len(ents)) {
		out = append(out, &span{kind: KindRelation, entities: ents, relations: c})
	}
	return out, nil
}

// task is one open question about a span: its fingerprint, or the next page
// of its remote listing.
type task struct {
	sp   *span
	list bool
	// limit is the page size of a listing.
	limit int
	// cursor is the last address of the previous page; nil on the first.
	cursor *string
	// remote holds the remote elements listed so far.
	remote []Element
	// local is the local fingerprint of the span, for a fingerprint task;
	// prepared reports that it is computed.
	local    Sum
	prepared bool
	// fam is the split t came from, when the remote count of the parent
	// is known.
	//
	// Nil: accepted. t was not split from a counted span.
	fam *family
}

// family checks that the remote counts of a split add up to the count the
// remote gave for the whole span. A remote that lies about counts to keep
// the client splitting, and so sending local addresses as range bounds, is
// caught at the first split it cannot account for.
type family struct {
	want, got uint64
	pending   int
	// broken is set when a child is answered by anything but a count, so
	// the sum cannot be checked.
	broken bool
}

// split returns tasks for spans as one family of a parent the remote
// counted want elements in.
func split(spans []*span, want uint64) []*task {
	f := &family{want: want, pending: len(spans)}
	out := tasksFor(spans)
	for _, t := range out {
		t.fam = f
	}
	return out
}

// counted records a child's remote count.
func (f *family) counted(n uint64) error {
	if f == nil || f.broken {
		return nil
	}
	f.got += min(n, ^uint64(0)-f.got)
	f.pending--
	if f.pending == 0 && f.got != f.want {
		return fmt.Errorf("%w: the parts of a range hold %d elements, the whole %d", ErrProtocol, f.got, f.want)
	}
	return nil
}

// abandon gives up the check of a child's family.
func (f *family) abandon() {
	if f != nil {
		f.broken = true
	}
}

// run is the state of one Diff call.
//
//nolint:containedctx // a run lives for one Diff call, and ctx is that call's
type run struct {
	d     *Differ
	ctx   context.Context
	stats Stats
	// remoteBytes counts the address bytes kept from the remote.
	remoteBytes int

	onlyLocal, onlyRemote []Element
	differing             []Change
	skipped               []Skipped
}

// round answers the open tasks: it computes the local side of every
// fingerprint task, sends the remote queries in packed requests, and returns
// the tasks still open.
func (r *run) round(work []*task) ([]*task, error) {
	var send, next []*task
	for _, t := range work {
		if t.list {
			send = append(send, t)
			continue
		}
		more, ready, err := r.prepare(t)
		if err != nil {
			return nil, err
		}
		next = append(next, more...)
		if ready {
			send = append(send, t)
		}
	}
	for len(send) > 0 {
		batch, rest := pack(send)
		send = rest
		answers, err := r.send(batch)
		var re *RangeError
		if errors.As(err, &re) && re.MetaGated {
			split, gErr := r.remoteGated(batch, re)
			if gErr != nil {
				return nil, gErr
			}
			next = append(next, split...)
			continue
		}
		if err != nil {
			return nil, err
		}
		notRun := false
		for i, t := range batch {
			if answers[i].Status == reconwire.StatusNotRun {
				// The server stops at its deadline and answers every range
				// after it not_run. Only the first may have used the time;
				// the rest are asked again as they are.
				if notRun {
					next = append(next, t)
					continue
				}
				notRun = true
			}
			more, hErr := r.handle(t, answers[i])
			if hErr != nil {
				return nil, hErr
			}
			next = append(next, more...)
		}
	}
	return next, nil
}

// prepare computes the local fingerprint of t. It returns ready when t is to
// be sent; otherwise more replaces it (a split, or nothing when the type is
// skipped).
func (r *run) prepare(t *task) (more []*task, ready bool, err error) {
	sp := t.sp
	if t.prepared {
		return nil, true, nil
	}
	sum, fpErr := r.d.local.Sum(r.ctx, sp.types(), sp.storeRange(sp.lo))
	switch {
	case fpErr == nil:
		t.local, t.prepared = sum, true
		return nil, true, nil
	case errors.Is(fpErr, visibility.ErrRangeTooLarge):
		// The local reader loads each row to fingerprint it, and the span
		// holds more rows than it loads at once. Split it as for a remote
		// too_large: by type, else into buckets that fit, else list it.
		// The split follows the local size alone.
		t.fam.abandon()
		more, err := r.tooLarge(sp)
		return more, false, err
	case errors.Is(fpErr, visibility.ErrRelationMetaGated) && sp.kind == KindRelation:
		t.fam.abandon()
		if sp.multiType() {
			return tasksFor(sp.byType()), false, nil
		}
		r.skipped = append(r.skipped, Skipped{
			Kind: KindRelation, Type: sp.relations[0],
			Reason: "the local policy can hide meta of this relation type",
		})
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("reconclient: local fingerprint: %w", fpErr)
	}
}

// remoteGated handles a meta gated refusal of the range re.Index of batch:
// the range is split by type, or its single type skipped, and every other
// task of the batch is asked again.
func (r *run) remoteGated(batch []*task, re *RangeError) ([]*task, error) {
	idx := re.Index
	if idx < 0 {
		// A remote may not name the range. With one relation range in the
		// request it can only be that one.
		idx = soleRelation(batch)
		if idx < 0 {
			return nil, fmt.Errorf("%w: the remote refused a range without naming it: %w", ErrProtocol, re)
		}
	}
	if idx >= len(batch) || batch[idx].sp.kind != KindRelation {
		return nil, fmt.Errorf("%w: a refused range the request does not hold: %w", ErrProtocol, re)
	}
	out := slices.Concat(batch[:idx], batch[idx+1:])
	batch[idx].fam.abandon()
	sp := batch[idx].sp
	if sp.multiType() {
		return append(out, tasksFor(sp.byType())...), nil
	}
	r.skipped = append(r.skipped, Skipped{
		Kind: KindRelation, Type: sp.relations[0], Remote: true, Reason: re.Detail,
	})
	return out, nil
}

// soleRelation returns the index of the one relation task of batch, or -1.
func soleRelation(batch []*task) int {
	idx := -1
	for i, t := range batch {
		if t.sp.kind == KindRelation {
			if idx >= 0 {
				return -1
			}
			idx = i
		}
	}
	return idx
}

// handle applies the answer to t and returns the tasks that follow from it.
func (r *run) handle(t *task, res reconwire.Result) ([]*task, error) {
	switch res.Status {
	case reconwire.StatusOK:
		if t.list {
			return r.page(t, res)
		}
		return r.compare(t, res)
	case reconwire.StatusTooLarge:
		if t.list {
			return nil, fmt.Errorf("%w: too_large for a list", ErrProtocol)
		}
		t.fam.abandon()
		return r.tooLarge(t.sp)
	case reconwire.StatusNotRun:
		t.sp.notRun++
		if t.sp.notRun > r.d.opt.MaxNotRun {
			return nil, fmt.Errorf("%w: the remote did not run a range %d times", ErrBudget, t.sp.notRun)
		}
		if t.sp.notRun == 1 {
			return []*task{t}, nil
		}
		if t.list {
			t.limit = max(t.limit/2, 1)
			return []*task{t}, nil
		}
		t.fam.abandon()
		return r.tooLarge(t.sp)
	default:
		return nil, fmt.Errorf("%w: status %q", ErrProtocol, Sanitize(res.Status))
	}
}

// compare checks a fingerprint answer against the local side.
func (r *run) compare(t *task, res reconwire.Result) ([]*task, error) {
	if res.Count == nil {
		return nil, fmt.Errorf("%w: fingerprint without a count", ErrProtocol)
	}
	if !validFingerprint(res.Fingerprint) {
		return nil, fmt.Errorf("%w: a fingerprint that is not %d lower-case hex digits", ErrProtocol, fingerprintLen)
	}
	if *res.Count == 0 && res.Fingerprint != emptyFingerprint {
		return nil, fmt.Errorf("%w: an empty range with a fingerprint", ErrProtocol)
	}
	if err := t.fam.counted(*res.Count); err != nil {
		return nil, err
	}
	remote := Sum{Count: *res.Count, Fingerprint: res.Fingerprint}
	if remote == t.local {
		return nil, nil
	}
	return r.mismatch(t.sp, remote.Count, t.local.Count)
}

// fingerprintLen is the length of a fingerprint on the wire: 128 bits in hex.
const fingerprintLen = 32

// emptyFingerprint is the fingerprint of the empty set.
var emptyFingerprint = strings.Repeat("0", fingerprintLen)

// validFingerprint reports whether s is a fingerprint in its one wire form,
// so that two equal sums are equal strings.
func validFingerprint(s string) bool {
	if len(s) != fingerprintLen {
		return false
	}
	for i := range len(s) {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// mismatch decides what to do with a span whose fingerprints differ.
//
// A multi-type span that is not small is split by type first, so the types
// are listed or split on their own and their pages share requests. A span
// with few local elements is listed, never split, so the remote learns no
// local address of it.
func (r *run) mismatch(sp *span, remoteCount, localCount uint64) ([]*task, error) {
	small := remoteCount <= listThreshold && localCount <= listThreshold-remoteCount
	switch {
	case remoteCount == 0:
		if err := r.enumerate(sp); err != nil {
			return nil, err
		}
		r.onlyLocal = append(r.onlyLocal, sp.local...)
		return nil, nil
	case sp.multiType() && !small:
		return split(sp.byType(), remoteCount), nil
	case localCount <= listThreshold:
		return []*task{listTask(sp, remoteCount)}, nil
	}
	return r.bucket(sp, fanout, remoteCount)
}

// tooLarge handles a span the remote would not fingerprint. A multi-type
// span is split by type. A single-type span with more than bucketRows local
// elements is split once, into buckets of at most bucketRows. Any other span
// is listed, however many rows the remote holds.
//
// The remote gives no count with too_large, so the count check of a split
// does not apply here. Splitting only above bucketRows, into parts that are
// listed if they are too_large again, bounds what a remote learns this way:
// one local address per bucketRows local elements, at one level.
func (r *run) tooLarge(sp *span) ([]*task, error) {
	if sp.multiType() {
		return tasksFor(sp.byType()), nil
	}
	if err := r.enumerate(sp); err != nil {
		return nil, err
	}
	if len(sp.local) > bucketRows {
		if parts := sp.byAddress((len(sp.local) + bucketRows - 1) / bucketRows); parts != nil {
			return tasksFor(parts), nil
		}
	}
	return []*task{listTask(sp, 0)}, nil
}

// bucket splits sp, which the remote counted want elements in, at its local
// element boundaries, or lists it when the local side has too few elements
// to split on.
func (r *run) bucket(sp *span, n int, want uint64) ([]*task, error) {
	if err := r.enumerate(sp); err != nil {
		return nil, err
	}
	if parts := sp.byAddress(n); parts != nil {
		return split(parts, want), nil
	}
	return []*task{listTask(sp, want)}, nil
}

// page applies one page of a remote listing, and diffs the span when the
// listing is complete.
func (r *run) page(t *task, res reconwire.Result) ([]*task, error) {
	sp := t.sp
	if len(res.Elements) > t.limit {
		return nil, fmt.Errorf("%w: %d elements for a limit of %d", ErrProtocol, len(res.Elements), t.limit)
	}
	floor, strict := sp.lo, false
	if t.cursor != nil {
		floor, strict = *t.cursor, true
	}
	if r.stats.RemoteElements += len(res.Elements); r.stats.RemoteElements > r.d.opt.MaxRemoteElements {
		return nil, fmt.Errorf("%w: more than %d remote elements", ErrBudget, r.d.opt.MaxRemoteElements)
	}
	for _, e := range res.Elements {
		if err := sp.checkAddress(e.Address, floor, strict); err != nil {
			return nil, err
		}
		if r.remoteBytes += len(e.Address); r.remoteBytes > r.d.opt.MaxRemoteBytes {
			return nil, fmt.Errorf("%w: more than %d bytes of remote addresses", ErrBudget, r.d.opt.MaxRemoteBytes)
		}
		h, err := canonical.ParseDigest(e.Hash)
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrProtocol, e.Address, err)
		}
		t.remote = append(t.remote, Element{Kind: sp.kind, Address: e.Address, Hash: h})
		floor, strict = e.Address, true
	}
	if res.Last != "" {
		// Last may be past the last element (a row dropped on load) but
		// not before it, and stays in the range. With no element it must
		// still pass the cursor.
		if err := sp.checkLast(res.Last, floor, len(res.Elements) == 0 && strict); err != nil {
			return nil, err
		}
	}
	if !res.More {
		if err := r.enumerate(sp); err != nil {
			return nil, err
		}
		r.merge(sp.local, t.remote)
		return nil, nil
	}
	// A page that stops at the limit must move the cursor, or paging
	// would never end.
	if res.Last == "" || (t.cursor != nil && res.Last <= *t.cursor) || res.Last < sp.lo {
		return nil, fmt.Errorf("%w: a page that does not advance", ErrProtocol)
	}
	last := res.Last
	t.cursor = &last
	return []*task{t}, nil
}

// checkLast checks the Last of a page: in the range, at or after floor, and
// short enough that the bound of the next page, one byte longer, is valid.
func (s *span) checkLast(last, floor string, strict bool) error {
	if len(last) >= store.MaxRangeBound || !Printable(last) {
		return fmt.Errorf("%w: invalid page cursor", ErrProtocol)
	}
	if (strict && last <= floor) || (!strict && last < floor) || (s.bounded && last >= s.hi) {
		return fmt.Errorf("%w: page cursor %q is out of the range", ErrProtocol, last)
	}
	return nil
}

// enumerate lists the local elements of sp, once.
func (r *run) enumerate(sp *span) error {
	if sp.localKnown {
		return nil
	}
	var els []Element
	lo, prev := sp.lo, ""
	for {
		page, err := r.d.local.List(r.ctx, sp.types(), sp.storeRange(lo), localPage)
		if err != nil {
			return fmt.Errorf("reconclient: local list: %w", err)
		}
		for _, row := range page.Rows {
			els = append(els, Element{Kind: sp.kind, Address: row.Address, Hash: row.Hash})
		}
		if r.stats.LocalElements += len(page.Rows); r.stats.LocalElements > r.d.opt.MaxLocalElements {
			return fmt.Errorf("%w: more than %d local elements", ErrBudget, r.d.opt.MaxLocalElements)
		}
		if !page.More {
			break
		}
		if page.Last == "" || page.Last <= prev {
			return errors.New("reconclient: local list did not advance")
		}
		if len(page.Last) >= store.MaxRangeBound {
			return fmt.Errorf("reconclient: a local address of %d bytes is too long to page past", len(page.Last))
		}
		prev = page.Last
		// No address holds NUL, so the next address is at least Last+"\x01".
		lo = page.Last + "\x01"
	}
	sp.local, sp.localKnown = els, true
	return nil
}

// merge diffs the local and remote elements of one span, both in order.
func (r *run) merge(local, remote []Element) {
	i, j := 0, 0
	for i < len(local) && j < len(remote) {
		l, m := local[i], remote[j]
		switch {
		case l.Address < m.Address:
			r.onlyLocal = append(r.onlyLocal, l)
			i++
		case l.Address > m.Address:
			r.onlyRemote = append(r.onlyRemote, m)
			j++
		default:
			if l.Hash != m.Hash {
				r.differing = append(r.differing,
					Change{Kind: l.Kind, Address: l.Address, Local: l.Hash, Remote: m.Hash})
			}
			i++
			j++
		}
	}
	r.onlyLocal = append(r.onlyLocal, local[i:]...)
	r.onlyRemote = append(r.onlyRemote, remote[j:]...)
}

// send sends one batch, waiting and sending again on an admission refusal.
func (r *run) send(batch []*task) ([]reconwire.Result, error) {
	req := reconwire.Request{Ranges: make([]reconwire.Range, len(batch))}
	for i, t := range batch {
		if t.list {
			req.Ranges[i] = t.sp.wire("list", t.limit, t.cursor)
		} else {
			req.Ranges[i] = t.sp.wire("fingerprint", 0, nil)
		}
	}
	for refusals := 0; ; refusals++ {
		if r.stats.RoundTrips >= r.d.opt.MaxRoundTrips {
			return nil, fmt.Errorf("%w: %d round trips", ErrBudget, r.stats.RoundTrips)
		}
		r.stats.RoundTrips++
		resp, err := r.d.remote.Reconcile(r.ctx, req)
		var ref *RefusedError
		if errors.As(err, &ref) {
			if refusals+1 >= r.d.opt.MaxRefusals {
				return nil, fmt.Errorf("%w: refused %d times in a row: %w", ErrBudget, refusals+1, err)
			}
			wait := min(max(ref.RetryAfter, time.Second), r.d.opt.MaxRetryWait)
			if sErr := r.d.opt.Sleep(r.ctx, wait); sErr != nil {
				return nil, fmt.Errorf("reconclient: waiting to retry: %w", sErr)
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(resp.Ranges) != len(batch) {
			return nil, fmt.Errorf("%w: %d answers for %d ranges", ErrProtocol, len(resp.Ranges), len(batch))
		}
		return resp.Ranges, nil
	}
}

// pack takes the leading tasks that fit one request under the server's
// limits and returns them and the rest. The first task always fits.
func pack(tasks []*task) (batch, rest []*task) {
	names, rows := 0, 0
	for i, t := range tasks {
		n, l := t.sp.names(), 0
		if t.list {
			l = t.limit
		}
		if i > 0 && (i == serverMaxRanges || names+n > serverMaxTypeNames || rows+l > serverMaxListRows) {
			return tasks[:i], tasks[i:]
		}
		names, rows = names+n, rows+l
	}
	return tasks, nil
}

// listTask returns a task that lists sp, its first page sized for about
// count rows.
func listTask(sp *span, count uint64) *task {
	limit := remotePage
	if count > 0 && count < remotePage-pageSlack {
		limit = int(count) + pageSlack
	}
	return &task{sp: sp, list: true, limit: limit}
}

func tasksFor(spans []*span) []*task {
	out := make([]*task, len(spans))
	for i, s := range spans {
		out[i] = &task{sp: s}
	}
	return out
}

// result returns the sets in (Kind, Address) order.
func (r *run) result() Result {
	byElement := func(a, b Element) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Address, b.Address))
	}
	slices.SortFunc(r.onlyLocal, byElement)
	slices.SortFunc(r.onlyRemote, byElement)
	slices.SortFunc(r.differing, func(a, b Change) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Address, b.Address))
	})
	slices.SortFunc(r.skipped, func(a, b Skipped) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(boolRank(a.Remote), boolRank(b.Remote)))
	})
	return Result{
		OnlyLocal: r.onlyLocal, OnlyRemote: r.onlyRemote, Differing: r.differing, Skipped: r.skipped,
		Stats: r.stats,
	}
}

func boolRank(b bool) int {
	if b {
		return 1
	}
	return 0
}
