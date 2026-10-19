package realtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// The pace a host runs [Triggers] at. They are re-exported because the
// wiring package may not import realtimelimits (arch-lint).
const (
	// TriggerTick is how often a host runs [Triggers.RunRound] while
	// [Triggers.Pending] holds.
	TriggerTick = realtimelimits.TriggerTick
	// TriggerReconcile is how often a host runs [Triggers.RunReconcile].
	TriggerReconcile = realtimelimits.TriggerReconcile
)

// TriggerDeps are the collaborators of [Triggers].
type TriggerDeps struct {
	Store  channels.Store
	Config ConfigSource
	// Local is the process-local state the channel service uses: the
	// engine marks the channels it posts to there, so live subscribers in
	// this process see the posts.
	Local *Local
	Meta  *metamodel.Metamodel
	Rows  EntitySource
	Seen  KV
}

// Triggers is the trigger engine. It posts a trigger's message when an
// entity starts matching the trigger's rule (enter), stops matching it
// while it exists (leave), or changes a property the rule reads while it
// matches (change).
//
// The store reports which entities changed through [Triggers.Note], which
// any goroutine may call. One goroutine runs everything else: it runs
// [Triggers.RunRound] every [TriggerTick] while [Triggers.Pending] holds,
// and [Triggers.RunReconcile] at start and every [TriggerReconcile]. With
// several processes on one store only the leader runs them, and calls
// [Triggers.Reset] when it takes over and when it stops leading. What the
// engine has seen is kept in the [KV], so the next leader carries on.
//
// A rule that is new, or whose `when:` changed, first records what matches
// now without posting: triggers start from the moment they are configured.
// Delivery is at least once: a post comes before the record of it, and a
// crash in between posts again.
type Triggers struct {
	w     systemWriter
	cfg   ConfigSource
	meta  *metamodel.Metamodel
	rows  EntitySource
	seen  seenStore
	clock Clock

	mu sync.Mutex
	// pending holds, per entity type, the ids changed since the last
	// round, with when each was first noted.
	pending  map[string]map[string]time.Time
	npending int
	// full lists the types whose changes overflowed
	// [realtimelimits.MaxTriggerPending]; the next round compares them in
	// full.
	full map[string]bool
	// types are the entity types the rules of typesOf read, so Note drops
	// changes no rule cares about.
	types   map[string]bool
	typesOf typesKey
	typesOK bool

	// lead is the state of this process's time as leader. Only the
	// goroutine that runs rounds touches it, and Reset clears it.
	lead leaderState
}

// typesKey is the snapshot Triggers.types was computed for.
type typesKey struct {
	config   *realtimecfg.Compiled
	degraded bool
}

// leaderState is what the engine keeps in memory between rounds while it
// leads. It is not shared with other processes, so a new leader starts
// without it.
type leaderState struct {
	// deferred holds, per entity type, ids held back by a cooldown or a
	// retry.
	deferred map[string]map[string]deferral
	// queues hold, per rule id, the posts waiting for a round with room
	// under [realtimelimits.MaxSystemPostsPerRound], oldest first.
	queues map[string]*effectQueue
	queued int
	// lastPost is when each (rule, entity) last posted, for the cooldown.
	lastPost map[string]time.Time
	// failures counts the failed attempts of each (rule, entity) post.
	failures map[string]*failure
	// dirty holds the shards whose save failed. Rounds read them from here
	// rather than from the KV, so a post that was made is not made again
	// while this process leads.
	dirty map[string]*dirtyShard
	// readFailSince is when rounds started failing to read; zero while
	// they succeed.
	readFailSince time.Time
	// rebaseline is set when a rule's seen-set was found corrupt and
	// dropped, so the next round baselines it again.
	rebaseline bool
}

func newLeaderState() leaderState {
	return leaderState{
		deferred: map[string]map[string]deferral{}, queues: map[string]*effectQueue{},
		lastPost: map[string]time.Time{}, failures: map[string]*failure{}, dirty: map[string]*dirtyShard{},
	}
}

// deferral is when a held-back id becomes due, and when it was first
// noted, so it keeps its place in its rule's queue.
type deferral struct {
	due, noted time.Time
}

// failure is the retry state of one post.
type failure struct {
	first    time.Time
	next     time.Time
	attempts int
}

// dirtyShard is a shard whose save failed, saved again from next on.
type dirtyShard struct {
	rule     string
	shards   int
	data     seenShard
	since    time.Time
	next     time.Time
	attempts int
	logged   bool
}

// retryDelay is the wait after the given number of failed attempts: it
// doubles from [TriggerTick] up to [realtimelimits.TriggerRetryMaxDelay].
func retryDelay(attempts int) time.Duration {
	d := TriggerTick
	for range attempts - 1 {
		if d >= realtimelimits.TriggerRetryMaxDelay {
			break
		}
		d *= 2
	}
	return min(d, realtimelimits.TriggerRetryMaxDelay)
}

// NewTriggers returns the engine over d.
//
// Nil: every field of d is rejected.
func NewTriggers(d TriggerDeps) (*Triggers, error) {
	switch {
	case d.Store == nil:
		return nil, errNil("Store")
	case d.Config == nil:
		return nil, errNil("Config")
	case d.Local == nil:
		return nil, errNil("Local")
	case d.Meta == nil:
		return nil, errNil("Meta")
	case d.Rows == nil:
		return nil, errNil("Rows")
	case d.Seen == nil:
		return nil, errNil("Seen")
	}
	return &Triggers{
		w:   systemWriter{store: d.Store, local: d.Local, cfg: d.Config},
		cfg: d.Config, meta: d.Meta, rows: d.Rows, seen: seenStore{kv: d.Seen}, clock: d.Local.clock,
		pending: map[string]map[string]time.Time{}, full: map[string]bool{}, lead: newLeaderState(),
	}, nil
}

// TriggerReport is what one round or reconcile did.
type TriggerReport struct {
	// Posted counts the messages posted.
	Posted int
	// Moved counts the channels an auto rule closed or archived.
	Moved int
	// Skipped counts the posts left out because the channel was not open.
	Skipped int
	// Failed counts the posts given up.
	Failed int
	// Baselined counts the rules that recorded their matches for the first
	// time.
	Baselined int
}

// Note records that an entity changed. It never blocks and never fails: a
// change no rule reads is dropped, and past
// [realtimelimits.MaxTriggerPending] changes the type holding the most is
// compared in full at the next round instead.
func (t *Triggers) Note(c Change) {
	if c.EntityType == "" || c.ID == "" {
		return
	}
	snap := t.cfg.Load()
	now := t.clock()
	t.mu.Lock()
	defer t.mu.Unlock()
	if k := (typesKey{config: snap.Config, degraded: snap.Degraded}); !t.typesOK || t.typesOf != k {
		t.types = map[string]bool{}
		for _, ru := range rulesFor(snap, t.meta) {
			t.types[ru.entityType] = true
		}
		t.typesOf, t.typesOK = k, true
	}
	if t.types[c.EntityType] {
		t.addPending(c.EntityType, c.ID, now)
	}
}

// addPending adds id of typ to the pending changes; the caller holds mu.
func (t *Triggers) addPending(typ, id string, at time.Time) {
	if t.full[typ] {
		return
	}
	if _, ok := t.pending[typ][id]; ok {
		return
	}
	if t.npending >= realtimelimits.MaxTriggerPending {
		most := typ
		for other, ids := range t.pending {
			if len(ids) > len(t.pending[most]) {
				most = other
			}
		}
		t.full[most] = true
		t.npending -= len(t.pending[most])
		delete(t.pending, most)
		if most == typ {
			return
		}
	}
	if t.pending[typ] == nil {
		t.pending[typ] = map[string]time.Time{}
	}
	t.pending[typ][id] = at
	t.npending++
}

// Reset forgets every change noted and everything kept in memory between
// rounds: the queued posts, the deferred changes, the cooldowns, the retry
// counts and the shards not yet saved. A host calls it when this process
// takes over as leader and when it stops leading, so it never acts on
// state another leader has since moved past. The next reconcile finds what
// was dropped.
func (t *Triggers) Reset() {
	t.mu.Lock()
	t.pending, t.full, t.npending = map[string]map[string]time.Time{}, map[string]bool{}, 0
	t.mu.Unlock()
	t.lead = newLeaderState()
}

// Pending reports whether a round has work: changes, a type to compare in
// full, a queued post, or a deferred change that is due.
func (t *Triggers) Pending() bool {
	now := t.clock()
	t.mu.Lock()
	busy := t.npending > 0 || len(t.full) > 0
	t.mu.Unlock()
	if busy || t.lead.queued > 0 || t.lead.rebaseline {
		return true
	}
	for _, ids := range t.lead.deferred {
		for _, d := range ids {
			if !now.Before(d.due) {
				return true
			}
		}
	}
	return false
}

// take swaps out the pending changes and the due deferred ones, with when
// each was first noted.
func (t *Triggers) take(now time.Time) (work map[string]map[string]time.Time, full map[string]bool) {
	t.mu.Lock()
	work, full = t.pending, t.full
	t.pending, t.full, t.npending = map[string]map[string]time.Time{}, map[string]bool{}, 0
	t.mu.Unlock()
	for typ, ids := range t.lead.deferred {
		for id, d := range ids {
			if now.Before(d.due) {
				continue
			}
			addID(work, typ, id, d.noted)
			delete(ids, id)
		}
		if len(ids) == 0 {
			delete(t.lead.deferred, typ)
		}
	}
	return work, full
}

// putBack returns the work of a failed round to the pending changes.
func (t *Triggers) putBack(work map[string]map[string]time.Time, full map[string]bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for typ := range full {
		t.full[typ] = true
	}
	for typ, ids := range work {
		for id, at := range ids {
			t.addPending(typ, id, at)
		}
	}
}

// postpone holds id of typ, first noted at noted, back until due.
func (t *Triggers) postpone(typ, id string, due, noted time.Time) {
	if t.lead.deferred[typ] == nil {
		t.lead.deferred[typ] = map[string]deferral{}
	}
	cur, ok := t.lead.deferred[typ][id]
	if !ok {
		t.lead.deferred[typ][id] = deferral{due: due, noted: noted}
		return
	}
	if due.Before(cur.due) {
		cur.due = due
	}
	if noted.Before(cur.noted) {
		cur.noted = noted
	}
	t.lead.deferred[typ][id] = cur
}

// addID adds id of typ to m, keeping the earliest time it was noted.
func addID(m map[string]map[string]time.Time, typ, id string, at time.Time) {
	if m[typ] == nil {
		m[typ] = map[string]time.Time{}
	}
	if cur, ok := m[typ][id]; !ok || at.Before(cur) {
		m[typ][id] = at
	}
}

// oldestFirst returns the ids of m in the order they were noted, so a
// rule's queue is first come, first served.
func oldestFirst(m map[string]time.Time) []string {
	ids := sortedKeys(m)
	slices.SortStableFunc(ids, func(a, b string) int { return m[a].Compare(m[b]) })
	return ids
}

// prune drops the cooldowns that ended and the retry counts of posts that
// stopped failing long ago, such as one whose change no longer arises.
func (t *Triggers) prune(now time.Time) {
	maps.DeleteFunc(t.lead.lastPost, func(_ string, at time.Time) bool {
		return now.Sub(at) >= realtimelimits.TriggerCooldown
	})
	maps.DeleteFunc(t.lead.failures, func(_ string, f *failure) bool {
		return now.Sub(f.first) >= 2*realtimelimits.TriggerRetryWindow
	})
}

// RunRound handles the changes noted since the last round and posts what
// the queues hold. Without rules (realtime off or degraded) the changes
// are dropped: the next reconcile after the config comes back catches up.
// A round that cannot read is retried with its changes for
// [realtimelimits.TriggerRetryWindow]; then they are left to the next
// reconcile.
func (t *Triggers) RunRound(ctx context.Context) (TriggerReport, error) {
	snap, now := t.cfg.Load(), t.clock()
	work, full := t.take(now)
	t.prune(now)
	rules := rulesFor(snap, t.meta)
	if len(rules) == 0 {
		t.lead.queues, t.lead.queued, t.lead.deferred = map[string]*effectQueue{}, 0, map[string]map[string]deferral{}
		return TriggerReport{}, nil
	}
	if len(work) == 0 && len(full) == 0 && t.lead.queued == 0 && !t.lead.rebaseline {
		return TriggerReport{}, nil
	}
	t.lead.rebaseline = false
	r := t.newRound(ctx, snap, now, rules)
	err := r.run(work, full, false)
	if err == nil {
		t.lead.readFailSince = time.Time{}
		return r.rep, nil
	}
	if t.lead.readFailSince.IsZero() {
		t.lead.readFailSince = now
	}
	if ctx.Err() != nil || now.Sub(t.lead.readFailSince) < realtimelimits.TriggerRetryWindow {
		t.putBack(work, full)
	} else {
		slog.ErrorContext(ctx, "realtime: trigger round keeps failing; its changes are left to the next reconcile",
			"error", err)
		t.lead.readFailSince = time.Time{}
	}
	return r.rep, err
}

// RunReconcile compares every entity the rules read with what the engine
// has seen and acts on the difference. It catches changes that raised no
// event (a rule on today() that starts to hold, an event the store
// dropped, a change while another process led) and removes the seen-sets
// of rules that are gone. With realtime off or degraded it does nothing,
// so a config that fails to load never erases what was seen.
func (t *Triggers) RunReconcile(ctx context.Context) (TriggerReport, error) {
	snap, now := t.cfg.Load(), t.clock()
	if snap.Config == nil || snap.Degraded {
		return TriggerReport{}, nil
	}
	t.prune(now)
	rules := rulesFor(snap, t.meta)
	r := t.newRound(ctx, snap, now, rules)
	full := map[string]bool{}
	for i := range rules {
		full[rules[i].entityType] = true
	}
	err := r.run(map[string]map[string]time.Time{}, full, true)
	return r.rep, err
}

// effectQueue is one rule's posts waiting for room in a round, in the
// order their changes were first noted. A post is keyed by its seen-set
// ref, so a later change to the same entity replaces the waiting post in
// place rather than queueing a second one.
type effectQueue struct {
	// order is sorted by noted time. It may name refs that items no
	// longer holds; pop skips them.
	order []queuedRef
	items map[string]queued
	// byID lists the queued refs of each entity id.
	byID map[string]map[string]bool
}

type queuedRef struct {
	ref   string
	noted time.Time
}

// queued is a post that waits: what the rule makes of one entity, worked
// out against the seen-set, which stays as it was until the post is made.
type queued struct {
	typ   string
	on    realtimecfg.TriggerOn
	fp    string
	gone  bool // leave: the entry goes
	noted time.Time
}

func newEffectQueue() *effectQueue {
	return &effectQueue{items: map[string]queued{}, byID: map[string]map[string]bool{}}
}

// put queues it for ref and reports whether ref is new to the queue. A ref
// already queued keeps its place; a new one goes after every post noted no
// later than it.
func (q *effectQueue) put(ref string, it queued) bool {
	if cur, had := q.items[ref]; had {
		it.noted = cur.noted
		q.items[ref] = it
		return false
	}
	q.insert(ref, it)
	return true
}

func (q *effectQueue) insert(ref string, it queued) {
	q.items[ref] = it
	id := idOf(ref)
	if q.byID[id] == nil {
		q.byID[id] = map[string]bool{}
	}
	q.byID[id][ref] = true
	e := queuedRef{ref: ref, noted: it.noted}
	if n := len(q.order); n == 0 || !it.noted.Before(q.order[n-1].noted) {
		q.order = append(q.order, e)
		return
	}
	i, _ := slices.BinarySearchFunc(q.order, it.noted, func(e queuedRef, t time.Time) int {
		if e.noted.After(t) {
			return 1
		}
		return -1
	})
	q.order = slices.Insert(q.order, i, e)
}

func (q *effectQueue) forget(ref string) {
	delete(q.items, ref)
	id := idOf(ref)
	delete(q.byID[id], ref)
	if len(q.byID[id]) == 0 {
		delete(q.byID, id)
	}
}

// remove drops ref and reports whether it was queued.
func (q *effectQueue) remove(ref string) bool {
	if _, ok := q.items[ref]; !ok {
		return false
	}
	q.forget(ref)
	if len(q.order) > 2*len(q.items)+16 {
		q.order = slices.DeleteFunc(q.order, func(e queuedRef) bool {
			_, ok := q.items[e.ref]
			return !ok
		})
	}
	return true
}

func (q *effectQueue) pop() (string, queued, bool) {
	for len(q.order) > 0 {
		e := q.order[0]
		q.order = q.order[1:]
		if it, ok := q.items[e.ref]; ok && it.noted.Equal(e.noted) {
			q.forget(e.ref)
			return e.ref, it, true
		}
	}
	return "", queued{}, false
}

// pushFront puts a popped post back in its place, which is the head.
func (q *effectQueue) pushFront(ref string, it queued) {
	q.items[ref] = it
	id := idOf(ref)
	if q.byID[id] == nil {
		q.byID[id] = map[string]bool{}
	}
	q.byID[id][ref] = true
	q.order = slices.Insert(q.order, 0, queuedRef{ref: ref, noted: it.noted})
}

// queuedRefs returns the refs of id that wait in rule ruleID's queue.
func (t *Triggers) queuedRefs(ruleID, id string) []string {
	q := t.lead.queues[ruleID]
	if q == nil {
		return nil
	}
	return sortedKeys(q.byID[id])
}

// enqueue queues a post of rule ruleID. Past [realtimelimits.MaxTriggerQueue]
// it is not queued, and as its entry stays as it was, the next reconcile
// finds it again.
func (t *Triggers) enqueue(ruleID, ref string, it queued) {
	q := t.lead.queues[ruleID]
	if q == nil {
		if t.lead.queued >= realtimelimits.MaxTriggerQueue {
			return
		}
		q = newEffectQueue()
		t.lead.queues[ruleID] = q
	}
	if _, had := q.items[ref]; !had && t.lead.queued >= realtimelimits.MaxTriggerQueue {
		return
	}
	if q.put(ref, it) {
		t.lead.queued++
	}
}

func (t *Triggers) dequeue(ruleID, ref string) {
	if q := t.lead.queues[ruleID]; q != nil && q.remove(ref) {
		t.lead.queued--
	}
}

// forgetRule drops what this process holds in memory for a rule's
// seen-set: its queue and its unsaved shards.
func (t *Triggers) forgetRule(ruleID string) {
	if q := t.lead.queues[ruleID]; q != nil {
		t.lead.queued -= len(q.items)
		delete(t.lead.queues, ruleID)
	}
	maps.DeleteFunc(t.lead.dirty, func(_ string, d *dirtyShard) bool { return d.rule == ruleID })
}

// saveTimeout bounds the saves at the end of a round. They run even when
// the round's context is canceled, so a post made just before shutdown is
// recorded rather than made again by the next leader.
const saveTimeout = 10 * time.Second

// rowsChunk is how many ids a round reads in one call to [EntitySource.Rows].
const rowsChunk = 1000

// errRuleReset is returned by round.shard for a rule whose seen-set was
// found corrupt and dropped; the caller skips the rule for this round.
var errRuleReset = errors.New("realtime: trigger seen-set reset")

// round is the state of one round or reconcile: one config snapshot, one
// clock reading, and the shards it has read.
//
//nolint:containedctx // a round lives inside one RunRound or RunReconcile call, which owns ctx
type round struct {
	t     *Triggers
	ctx   context.Context
	snap  Snapshot
	now   time.Time
	rules []rule
	byID  map[string]*rule
	// b holds the functions rules call, bound once for the round.
	b      *predicate.Bindings
	idx    seenIndex
	shards map[string]*roundShard
	rep    TriggerReport
	// scanned lists the rules whose shards a full scan has read.
	scanned map[string]bool
}

// roundShard is a shard a round has read.
type roundShard struct {
	rule    string
	shards  int
	data    seenShard
	changed bool
	// byID lists the refs of each entity id, for a rule that reads each
	// face; built on first use.
	byID map[string][]string
}

func (t *Triggers) newRound(ctx context.Context, snap Snapshot, now time.Time, rules []rule) *round {
	r := &round{t: t, ctx: ctx, snap: snap, now: now, rules: rules, byID: map[string]*rule{},
		shards: map[string]*roundShard{}, scanned: map[string]bool{}}
	for i := range rules {
		r.byID[rules[i].id] = &rules[i]
	}
	return r
}

// run baselines new rules, adds the candidates of every type in full to
// work, queues the posts work calls for, posts what the queues hold and
// saves what changed. A reconcile also removes the seen-sets of rules
// that are gone and splits rules that grew.
func (r *round) run(work map[string]map[string]time.Time, full map[string]bool, reconcile bool) error {
	r.b = predicate.NewBindings()
	if err := predicatefns.Bind(r.b, r.now); err != nil {
		return err
	}
	idx, err := r.t.seen.loadIndex(r.ctx)
	var corrupt corruptError
	if errors.As(err, &corrupt) {
		slog.ErrorContext(r.ctx, "realtime: the trigger index does not decode; every trigger starts over "+
			"from what matches now", "error", err)
		for id := range r.t.lead.queues {
			r.t.forgetRule(id)
		}
		r.t.lead.dirty = map[string]*dirtyShard{}
		// The KV cannot list keys, so the shards of the current rules are
		// deleted under every layout they could have had. Those of rules
		// removed since are not known and stay behind.
		for i := range r.rules {
			if dropErr := r.t.seen.dropAllLayouts(r.ctx, r.rules[i].id); dropErr != nil {
				slog.WarnContext(r.ctx, "realtime: could not delete an old trigger seen-set",
					"rule", r.rules[i].name, "error", dropErr)
			}
		}
		err = nil
	}
	if err != nil {
		return err
	}
	r.idx = idx
	if err := r.baseline(); err != nil {
		return err
	}
	if reconcile {
		if err := r.collect(); err != nil {
			return err
		}
	}
	for _, typ := range sortedKeys(full) {
		cands, err := r.candidates(typ)
		if err != nil {
			return err
		}
		for id := range cands {
			addID(work, typ, id, r.now)
		}
	}
	for _, typ := range sortedKeys(work) {
		if err := r.process(typ, oldestFirst(work[typ]), work[typ]); err != nil {
			return err
		}
	}
	r.post()
	r.save(r.ctx)
	if reconcile {
		r.reshard()
	}
	return nil
}

// active returns the rules that have a seen-set, for type typ when typ is
// set.
func (r *round) active(typ string) []*rule {
	var out []*rule
	for i := range r.rules {
		ru := &r.rules[i]
		if _, ok := r.idx.Rules[ru.id]; ok && (typ == "" || ru.entityType == typ) {
			out = append(out, ru)
		}
	}
	return out
}

// baseline records the current matches of every rule without a seen-set,
// without posting, and adds the rules to the index last. A rule whose
// baseline fails is tried again next round.
func (r *round) baseline() error {
	groups := map[groupKey][]*rule{}
	for i := range r.rules {
		ru := &r.rules[i]
		if _, ok := r.idx.Rules[ru.id]; !ok {
			k := groupKey{typ: ru.entityType, faces: ru.faces}
			groups[k] = append(groups[k], ru)
		}
	}
	if len(groups) == 0 {
		return nil
	}
	added := 0
	for _, k := range sortedGroups(groups) {
		rules := groups[k]
		entries := make([]seenShard, len(rules))
		for i := range entries {
			entries[i] = seenShard{}
		}
		err := r.t.rows.Scan(r.ctx, k.typ, k.faces, func(rows []EntityRow) error {
			for _, row := range rows {
				for i, ru := range rules {
					if match, fp, ok := ru.eval(r.ctx, r.t.meta, r.b, row); ok && match {
						entries[i][ru.ref(row)] = fp
					}
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("realtime: baseline %s: %w", k.typ, err)
		}
		for i, ru := range rules {
			r.t.forgetRule(ru.id)
			shards := shardsFor(len(entries[i]))
			if err := r.writeLayout(ru.id, shards, entries[i]); err != nil {
				slog.WarnContext(r.ctx, "realtime: trigger baseline not saved; trying again next round",
					"rule", ru.name, "error", err)
				continue
			}
			r.idx.Rules[ru.id] = seenRule{Shards: shards}
			added++
		}
	}
	if added == 0 {
		return nil
	}
	if err := r.t.seen.saveIndex(r.ctx, r.idx); err != nil {
		return err
	}
	r.rep.Baselined += added
	return nil
}

// writeLayout writes entries as the shards of a rule split over shards.
func (r *round) writeLayout(ruleID string, shards int, entries seenShard) error {
	parts := make([]seenShard, shards)
	for i := range parts {
		parts[i] = seenShard{}
	}
	for ref, fp := range entries {
		parts[shardOf(idOf(ref), shards)][ref] = fp
	}
	for n, part := range parts {
		if err := r.t.seen.saveShard(r.ctx, shardKey(ruleID, shards, n), part); err != nil {
			return err
		}
	}
	return nil
}

// collect removes the seen-sets of rules the config no longer has: index
// entries first, so a crash leaves unreferenced shards rather than an
// index naming deleted ones.
func (r *round) collect() error {
	gone := map[string]seenRule{}
	for id, meta := range r.idx.Rules {
		if r.byID[id] == nil {
			gone[id] = meta
			delete(r.idx.Rules, id)
		}
	}
	for id := range r.t.lead.queues {
		if r.byID[id] == nil {
			r.t.forgetRule(id)
		}
	}
	if len(gone) == 0 {
		return nil
	}
	if err := r.t.seen.saveIndex(r.ctx, r.idx); err != nil {
		return err
	}
	for _, id := range sortedKeys(gone) {
		r.t.forgetRule(id)
		if err := r.t.seen.dropShards(r.ctx, id, gone[id].Shards); err != nil {
			slog.WarnContext(r.ctx, "realtime: could not delete the seen-set of a removed trigger",
				"rule", id, "error", err)
		}
	}
	return nil
}

// shard returns shard n of ru, read once per round. A shard whose save
// failed in an earlier round is read from memory. A shard that does not
// decode resets ru (see round.reset) and returns errRuleReset.
func (r *round) shard(ru *rule, n int) (*roundShard, error) {
	shards := r.idx.Rules[ru.id].Shards
	key := shardKey(ru.id, shards, n)
	if sh, ok := r.shards[key]; ok {
		return sh, nil
	}
	if d, ok := r.t.lead.dirty[key]; ok && d.shards == shards {
		sh := &roundShard{rule: ru.id, shards: shards, data: d.data, changed: true}
		r.shards[key] = sh
		return sh, nil
	}
	data, err := r.t.seen.loadShard(r.ctx, key)
	var corrupt corruptError
	if errors.As(err, &corrupt) {
		r.reset(ru, err)
		return nil, errRuleReset
	}
	if err != nil {
		return nil, err
	}
	sh := &roundShard{rule: ru.id, shards: shards, data: data}
	r.shards[key] = sh
	return sh, nil
}

// reset drops the seen-set of ru, whose shard does not decode, so the
// next round baselines it again without posting. The other rules carry
// on. A change to ru's entities between now and that baseline posts
// nothing.
func (r *round) reset(ru *rule, cause error) {
	slog.ErrorContext(r.ctx, "realtime: a trigger seen-set does not decode; the trigger starts over from "+
		"what matches now", "rule", ru.name, "error", cause)
	meta := r.idx.Rules[ru.id]
	delete(r.idx.Rules, ru.id)
	for n := range meta.Shards {
		delete(r.shards, shardKey(ru.id, meta.Shards, n))
	}
	r.t.forgetRule(ru.id)
	r.t.lead.rebaseline = true
	if err := r.t.seen.saveIndex(r.ctx, r.idx); err != nil {
		slog.WarnContext(r.ctx, "realtime: could not save the trigger index", "error", err)
		return
	}
	if err := r.t.seen.dropShards(r.ctx, ru.id, meta.Shards); err != nil {
		slog.WarnContext(r.ctx, "realtime: could not delete a corrupt trigger seen-set", "rule", ru.name,
			"error", err)
	}
}

// refsOf returns the seen-set keys sh holds for entity id under ru.
func (sh *roundShard) refsOf(ru *rule, id string) []string {
	if ru.faces == FacePrime {
		if _, ok := sh.data[id]; ok {
			return []string{id}
		}
		return nil
	}
	if sh.byID == nil {
		sh.byID = map[string][]string{}
		for ref := range sh.data {
			i := idOf(ref)
			sh.byID[i] = append(sh.byID[i], ref)
		}
	}
	return sh.byID[id]
}

func (sh *roundShard) set(ref, fp string) {
	if cur, ok := sh.data[ref]; ok && cur == fp {
		return
	}
	if _, ok := sh.data[ref]; !ok && sh.byID != nil {
		i := idOf(ref)
		sh.byID[i] = append(sh.byID[i], ref)
	}
	sh.data[ref] = fp
	sh.changed = true
}

func (sh *roundShard) drop(ref string) {
	if _, ok := sh.data[ref]; !ok {
		return
	}
	delete(sh.data, ref)
	if sh.byID != nil {
		i := idOf(ref)
		sh.byID[i] = slices.DeleteFunc(sh.byID[i], func(s string) bool { return s == ref })
	}
	sh.changed = true
}

func (sh *roundShard) apply(ref string, it queued) {
	if it.gone {
		sh.drop(ref)
		return
	}
	sh.set(ref, it.fp)
}

// candidates scans every entity of typ and returns the ids whose state
// under some rule differs from what the engine has seen, including ids it
// has seen that no longer exist, and every id with a post waiting in a
// queue, whose change may have been undone by an event that was lost. It
// reads rows without acting on them: the round reads the candidates again
// by id and acts on what it reads then.
func (r *round) candidates(typ string) (map[string]bool, error) {
	out := map[string]bool{}
	groups := map[FaceMode][]*rule{}
	for _, ru := range r.active(typ) {
		groups[ru.faces] = append(groups[ru.faces], ru)
		if q := r.t.lead.queues[ru.id]; q != nil {
			for id := range q.byID {
				out[id] = true
			}
		}
	}
	for _, faces := range []FaceMode{FacePrime, FaceEach} {
		if rules := groups[faces]; len(rules) > 0 {
			if err := r.compare(typ, faces, rules, out); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// compare is [round.candidates] for the rules that read the faces of one
// face mode, which share one scan.
func (r *round) compare(typ string, faces FaceMode, rules []*rule, out map[string]bool) error {
	rules, err := r.loadAll(rules)
	if err != nil || len(rules) == 0 {
		return err
	}
	present := make([]map[string]bool, len(rules))
	for i := range present {
		present[i] = map[string]bool{}
	}
	err = r.t.rows.Scan(r.ctx, typ, faces, func(rows []EntityRow) error {
		for _, row := range rows {
			for i, ru := range rules {
				ref := ru.ref(row)
				present[i][ref] = true
				if r.differs(ru, row, ref) {
					out[row.ID] = true
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("realtime: compare %s: %w", typ, err)
	}
	for i, ru := range rules {
		shards := r.idx.Rules[ru.id].Shards
		for n := range shards {
			for ref := range r.shards[shardKey(ru.id, shards, n)].data {
				if !present[i][ref] {
					out[idOf(ref)] = true
				}
			}
		}
		r.scanned[ru.id] = true
	}
	return nil
}

// loadAll reads every shard of rules and returns the rules whose
// seen-set was not reset on the way.
func (r *round) loadAll(rules []*rule) ([]*rule, error) {
	out := rules[:0:0]
	for _, ru := range rules {
		ok := true
		for n := range r.idx.Rules[ru.id].Shards {
			_, err := r.shard(ru, n)
			if errors.Is(err, errRuleReset) {
				ok = false
				break
			}
			if err != nil {
				return nil, err
			}
		}
		if ok {
			out = append(out, ru)
		}
	}
	return out, nil
}

// differs reports whether row's state under ru differs from what the
// engine has seen. The shard is already read, so this does no I/O.
func (r *round) differs(ru *rule, row EntityRow, ref string) bool {
	shards := r.idx.Rules[ru.id].Shards
	sh := r.shards[shardKey(ru.id, shards, shardOf(row.ID, shards))]
	match, fp, ok := ru.eval(r.ctx, r.t.meta, r.b, row)
	prev, had := sh.data[ref]
	return ok && (match != had || (match && fp != prev))
}

// process reads ids of typ in chunks, one read per face mode, works out
// each active rule's effect on each entity, applies the ones that post
// nothing and queues the rest.
func (r *round) process(typ string, ids []string, noted map[string]time.Time) error {
	for chunk := range slices.Chunk(ids, rowsChunk) {
		rules := r.active(typ)
		if len(rules) == 0 {
			return nil
		}
		rows := map[FaceMode]map[string][]EntityRow{}
		for _, ru := range rules {
			if rows[ru.faces] != nil {
				continue
			}
			got, err := r.t.rows.Rows(r.ctx, typ, chunk, ru.faces)
			if err != nil {
				return fmt.Errorf("realtime: read %s: %w", typ, err)
			}
			byID := map[string][]EntityRow{}
			for _, row := range got {
				byID[row.ID] = append(byID[row.ID], row)
			}
			rows[ru.faces] = byID
		}
		for _, ru := range rules {
			if err := r.effects(ru, typ, chunk, rows[ru.faces], noted); err != nil {
				return err
			}
		}
	}
	return nil
}

// effects works out ru's effect on each of ids against the seen-set. It
// applies the ones that post nothing and queues the ones that post,
// replacing a post of the same entity that still waits; an entity that
// came back to what was seen, or is gone, drops its waiting post.
func (r *round) effects(
	ru *rule, typ string, ids []string, rows map[string][]EntityRow, noted map[string]time.Time,
) error {
	shards := r.idx.Rules[ru.id].Shards
	for _, id := range ids {
		sh, err := r.shard(ru, shardOf(id, shards))
		if errors.Is(err, errRuleReset) {
			return nil
		}
		if err != nil {
			return err
		}
		cur := map[string]EntityRow{}
		for _, row := range rows[id] {
			cur[ru.ref(row)] = row
		}
		refs := r.refsFor(ru, sh, id, cur)
		for _, ref := range refs {
			row, exists := cur[ref]
			if !exists {
				sh.drop(ref) // deleted: no post
				r.t.dequeue(ru.id, ref)
				continue
			}
			match, fp, ok := ru.eval(r.ctx, r.t.meta, r.b, row)
			if !ok {
				continue
			}
			prev, had := sh.data[ref]
			it := queued{typ: typ, fp: fp, noted: noted[id]}
			switch {
			case match && !had:
				it.on = realtimecfg.TriggerEnter
			case match && fp != prev:
				it.on = realtimecfg.TriggerChange
			case !match && had:
				it.on, it.gone = realtimecfg.TriggerLeave, true
			default:
				r.t.dequeue(ru.id, ref)
				continue
			}
			if it.on != ru.on {
				sh.apply(ref, it)
				r.t.dequeue(ru.id, ref)
				continue
			}
			r.t.enqueue(ru.id, ref, it)
		}
	}
	return nil
}

// refsFor is every ref of id ru has to look at: the ones it has seen, the
// ones waiting in its queue and the ones the entity has now.
func (r *round) refsFor(ru *rule, sh *roundShard, id string, cur map[string]EntityRow) []string {
	refs := slices.Clone(sh.refsOf(ru, id))
	refs = append(refs, r.t.queuedRefs(ru.id, id)...)
	for ref := range cur {
		refs = append(refs, ref)
	}
	slices.Sort(refs)
	return slices.Compact(refs)
}

// post takes one post from each rule's queue in turn, in rule order, until
// [realtimelimits.MaxSystemPostsPerRound] attempts are spent. It stops
// early when the store fails or ctx ends; the rest wait in their queues.
// The queues of rules the config no longer has are dropped.
func (r *round) post() {
	for id := range r.t.lead.queues {
		if r.byID[id] == nil {
			r.t.forgetRule(id)
		}
	}
	ids := make([]string, 0, len(r.rules))
	for i := range r.rules {
		ids = append(ids, r.rules[i].id)
	}
	attempts := 0
	for attempts < realtimelimits.MaxSystemPostsPerRound {
		progressed := false
		for _, id := range ids {
			if attempts >= realtimelimits.MaxSystemPostsPerRound {
				break
			}
			q := r.t.lead.queues[id]
			if q == nil {
				continue
			}
			ref, it, ok := q.pop()
			if !ok {
				continue
			}
			r.t.lead.queued--
			progressed = true
			ru := r.byID[id]
			if _, active := r.idx.Rules[id]; ru == nil || !active {
				continue
			}
			tried, stop := r.postOne(ru, q, ref, it)
			if tried {
				attempts++
			}
			if stop {
				r.dropEmptyQueues()
				return
			}
		}
		if !progressed {
			break
		}
	}
	r.dropEmptyQueues()
}

func (r *round) dropEmptyQueues() {
	maps.DeleteFunc(r.t.lead.queues, func(_ string, q *effectQueue) bool { return len(q.items) == 0 })
}

// postOne makes one post popped from q. tried reports whether it reached
// the store, and stop that the round should post no more. A post held
// back by the cooldown or a retry delay leaves the queue and its entity
// is read again when it is due, so later changes are seen with it.
func (r *round) postOne(ru *rule, q *effectQueue, ref string, it queued) (tried, stop bool) {
	k := ru.id + "\x00" + ref
	id := idOf(ref)
	if last, ok := r.t.lead.lastPost[k]; ok && r.now.Sub(last) < realtimelimits.TriggerCooldown {
		r.t.postpone(it.typ, id, last.Add(realtimelimits.TriggerCooldown), it.noted)
		return false, false
	}
	f := r.t.lead.failures[k]
	if f != nil && r.now.Before(f.next) {
		r.t.postpone(it.typ, id, f.next, it.noted)
		return false, false
	}
	if ru.auto != nil && r.waitsForTrigger(ru, id) {
		r.t.postpone(it.typ, id, r.now.Add(TriggerTick), it.noted)
		return false, false
	}
	sh, err := r.shard(ru, shardOf(id, r.idx.Rules[ru.id].Shards))
	if errors.Is(err, errRuleReset) {
		return false, false
	}
	if err != nil {
		r.pushBack(q, ref, it)
		return false, true
	}
	err = r.act(ru, ref)
	var permanent permanentError
	switch {
	case err == nil:
		r.t.lead.lastPost[k] = r.now
	case errors.Is(err, channels.ErrNotOpen):
		r.rep.Skipped++
		slog.DebugContext(r.ctx, "realtime: trigger skipped; the channel is not open",
			"rule", ru.name, "entity", ref)
	case r.ctx.Err() != nil:
		r.pushBack(q, ref, it)
		return true, true
	case errors.As(err, &permanent):
		r.rep.Failed++
		slog.ErrorContext(r.ctx, "realtime: trigger post cannot succeed; the change is counted as seen",
			"rule", ru.name, "entity", ref, "error", err)
	default:
		if f == nil {
			f = &failure{first: r.now}
			r.t.lead.failures[k] = f
		}
		f.attempts++
		if r.now.Sub(f.first) < realtimelimits.TriggerRetryWindow {
			f.next = r.now.Add(retryDelay(f.attempts))
			r.t.postpone(it.typ, id, f.next, it.noted)
			return true, true
		}
		r.rep.Failed++
		slog.ErrorContext(r.ctx, "realtime: trigger post keeps failing; the change is counted as seen",
			"rule", ru.name, "entity", ref, "attempts", f.attempts, "error", err)
		stop = true
	}
	delete(r.t.lead.failures, k)
	sh.apply(ref, it)
	return true, stop
}

// act does what ru does on its edge for the entity ref: a trigger posts
// its message, an auto rule moves the entity's channels.
func (r *round) act(ru *rule, ref string) error {
	if ru.auto == nil {
		err := r.t.w.post(r.ctx, r.snap, r.now, ru.key(ref), ru.ct, ru.mt, ru.props,
			ru.triggeredBy(), ru.trigger.WhenClosed != realtimecfg.WhenClosedPost)
		if err == nil {
			r.rep.Posted++
		}
		return err
	}
	n, err := r.t.w.transition(r.ctx, r.now, ru.anchor(ref), ru.ct, ru.auto, ru.triggeredBy())
	r.rep.Moved += n
	return err
}

// waitsForTrigger reports whether a trigger into ru's channel type still
// has a post of entity id in its queue. That post goes first: were the
// auto rule to close the channel now, the post would find it closed.
func (r *round) waitsForTrigger(ru *rule, id string) bool {
	for i := range r.rules {
		o := &r.rules[i]
		if o.trigger != nil && o.ct.Name == ru.ct.Name && len(r.t.queuedRefs(o.id, id)) > 0 {
			return true
		}
	}
	return false
}

func (r *round) pushBack(q *effectQueue, ref string, it queued) {
	q.pushFront(ref, it)
	r.t.lead.queued++
}

// save writes every shard the round changed, and every shard an earlier
// round failed to save once its retry delay has passed. It runs on a
// context that outlives the round's, so a shutdown still records the
// posts it made. A failed save keeps the shard in memory, so this process
// does not post the same change again while it leads.
func (r *round) save(parent context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), saveTimeout)
	defer cancel()
	for _, key := range sortedKeys(r.shards) {
		sh := r.shards[key]
		if !sh.changed {
			continue
		}
		if d := r.t.lead.dirty[key]; d != nil && r.now.Before(d.next) {
			d.data = sh.data // not due: keep what the round added for the next try
			continue
		}
		r.saveShard(ctx, key, sh)
	}
	for _, key := range sortedKeys(r.t.lead.dirty) {
		d := r.t.lead.dirty[key]
		if _, done := r.shards[key]; done {
			continue
		}
		if meta, ok := r.idx.Rules[d.rule]; !ok || meta.Shards != d.shards {
			delete(r.t.lead.dirty, key) // the rule or its layout is gone
			continue
		}
		if r.now.Before(d.next) {
			continue
		}
		r.saveShard(ctx, key, &roundShard{rule: d.rule, shards: d.shards, data: d.data})
	}
}

func (r *round) saveShard(ctx context.Context, key string, sh *roundShard) {
	err := r.t.seen.saveShard(ctx, key, sh.data)
	if err == nil {
		delete(r.t.lead.dirty, key)
		return
	}
	d := r.t.lead.dirty[key]
	if d == nil {
		d = &dirtyShard{since: r.now}
		r.t.lead.dirty[key] = d
	}
	d.rule, d.shards, d.data = sh.rule, sh.shards, sh.data
	d.attempts++
	d.next = r.now.Add(retryDelay(d.attempts))
	if !d.logged && r.now.Sub(d.since) >= realtimelimits.TriggerRetryWindow {
		d.logged = true
		slog.ErrorContext(ctx, "realtime: the trigger seen-set cannot be saved; this process keeps it in "+
			"memory, and another process may post these changes again", "key", key, "error", err)
	}
}

// reshard splits or joins a rule whose entry count no longer fits its
// shard count. It runs after a reconcile has read and updated every shard.
// The new shards are written beside the old ones and the index switches
// over, so a crash in between leaves the old layout in force.
func (r *round) reshard() {
	for i := range r.rules {
		ru := &r.rules[i]
		meta, ok := r.idx.Rules[ru.id]
		if !r.scanned[ru.id] || !ok {
			continue
		}
		entries := seenShard{}
		for n := range meta.Shards {
			key := shardKey(ru.id, meta.Shards, n)
			sh, ok := r.shards[key]
			if !ok || r.t.lead.dirty[key] != nil {
				entries = nil
				break
			}
			maps.Copy(entries, sh.data)
		}
		shards := shardsFor(len(entries))
		if entries == nil || shards == meta.Shards {
			continue
		}
		if err := r.writeLayout(ru.id, shards, entries); err != nil {
			slog.WarnContext(r.ctx, "realtime: could not split a trigger seen-set", "rule", ru.name, "error", err)
			continue
		}
		r.idx.Rules[ru.id] = seenRule{Shards: shards}
		if err := r.t.seen.saveIndex(r.ctx, r.idx); err != nil {
			r.idx.Rules[ru.id] = meta
			slog.WarnContext(r.ctx, "realtime: could not split a trigger seen-set", "rule", ru.name, "error", err)
			continue
		}
		if err := r.t.seen.dropShards(r.ctx, ru.id, meta.Shards); err != nil {
			slog.WarnContext(r.ctx, "realtime: could not delete a trigger's old seen-set shards",
				"rule", ru.name, "error", err)
		}
	}
}

// groupKey groups rules that read the same rows.
type groupKey struct {
	typ   string
	faces FaceMode
}

func sortedGroups[V any](m map[groupKey]V) []groupKey {
	out := make([]groupKey, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.SortFunc(out, func(a, b groupKey) int {
		if a.typ != b.typ {
			return strings.Compare(a.typ, b.typ)
		}
		return int(a.faces) - int(b.faces)
	})
	return out
}
