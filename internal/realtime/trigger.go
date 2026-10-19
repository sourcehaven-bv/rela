package realtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// The pace a host runs [Triggers] at.
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
// The store reports which entities changed through [Triggers.Note]. A host
// runs [Triggers.RunRound] every [TriggerTick] while [Triggers.Pending]
// holds, and [Triggers.RunReconcile] at start and every
// [TriggerReconcile]. The two must not run at the same time,
// and with several processes on one store only one process may run them
// at a time: the host holds its leader lock around each. What the engine
// has seen is kept in the [KV], so the next leader carries on.
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
	// pending holds, per entity type, the ids changed since the last round.
	pending  map[string]map[string]bool
	npending int
	// full lists the types whose changes overflowed
	// [realtimelimits.MaxTriggerPending]; the next round compares them in
	// full.
	full map[string]bool
	// deferred holds, per entity type, ids held back by a trigger's
	// cooldown, with the time they become due.
	deferred map[string]map[string]time.Time
	// types are the entity types the rules of typesOf read, so Note drops
	// changes no rule cares about.
	types   map[string]bool
	typesOf *realtimecfg.Compiled
	typesOK bool

	// The rounds alone touch these, and rounds never overlap.
	lastPost  map[string]time.Time
	failures  map[string]int
	dirty     map[string]*dirtyShard
	readFails int
}

// dirtyShard is a shard whose save failed. Rounds read it from here rather
// than from the KV, so a post that was made is never made again, and save
// it again until a save succeeds.
type dirtyShard struct {
	data  seenShard
	fails int
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
		pending: map[string]map[string]bool{}, full: map[string]bool{},
		deferred: map[string]map[string]time.Time{},
		lastPost: map[string]time.Time{}, failures: map[string]int{}, dirty: map[string]*dirtyShard{},
	}, nil
}

// TriggerReport is what one round or reconcile did.
type TriggerReport struct {
	// Posted counts the messages posted.
	Posted int
	// Skipped counts the posts left out because the channel was not open.
	Skipped int
	// Failed counts the posts given up after [realtimelimits.TriggerRetries].
	Failed int
	// Baselined counts the rules that recorded their matches for the first
	// time.
	Baselined int
}

// Note records that an entity changed. It never blocks and never fails: a
// change no rule reads is dropped, and past
// [realtimelimits.MaxTriggerPending] changes the type is compared in full
// at the next round.
func (t *Triggers) Note(c Change) {
	if c.EntityType == "" || c.ID == "" {
		return
	}
	snap := t.cfg.Load()
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.typesOK || t.typesOf != snap.Config {
		t.types = map[string]bool{}
		if !snap.Degraded {
			for _, tr := range snap.Config.Triggers() {
				t.types[tr.EntityType] = true
			}
		}
		t.typesOf, t.typesOK = snap.Config, true
	}
	if !t.types[c.EntityType] || t.full[c.EntityType] {
		return
	}
	ids := t.pending[c.EntityType]
	if ids[c.ID] {
		return
	}
	if t.npending >= realtimelimits.MaxTriggerPending {
		t.full[c.EntityType] = true
		t.npending -= len(ids)
		delete(t.pending, c.EntityType)
		return
	}
	if ids == nil {
		ids = map[string]bool{}
		t.pending[c.EntityType] = ids
	}
	ids[c.ID] = true
	t.npending++
}

// Drop forgets the changes noted since the last round. A host that is not
// the one running rounds calls it, so they do not pile up.
func (t *Triggers) Drop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending, t.full, t.npending = map[string]map[string]bool{}, map[string]bool{}, 0
}

// Pending reports whether a round has work: changes, a type to compare in
// full, or a deferred change that is due.
func (t *Triggers) Pending() bool {
	now := t.clock()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.npending > 0 || len(t.full) > 0 {
		return true
	}
	for _, ids := range t.deferred {
		for _, due := range ids {
			if !now.Before(due) {
				return true
			}
		}
	}
	return false
}

// take swaps out the pending changes and the due deferred ones.
func (t *Triggers) take(now time.Time) (work map[string]map[string]bool, full map[string]bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	work, full = t.pending, t.full
	t.pending, t.full, t.npending = map[string]map[string]bool{}, map[string]bool{}, 0
	for typ, ids := range t.deferred {
		for id, due := range ids {
			if now.Before(due) {
				continue
			}
			addID(work, typ, id)
			delete(ids, id)
		}
		if len(ids) == 0 {
			delete(t.deferred, typ)
		}
	}
	return work, full
}

// requeue puts ids of typ back for the next round.
func (t *Triggers) requeue(typ string, ids ...string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, id := range ids {
		if t.pending[typ][id] {
			continue
		}
		addID(t.pending, typ, id)
		t.npending++
	}
}

// postpone holds id of typ back until due.
func (t *Triggers) postpone(typ, id string, due time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.deferred[typ] == nil {
		t.deferred[typ] = map[string]time.Time{}
	}
	if cur, ok := t.deferred[typ][id]; !ok || due.Before(cur) {
		t.deferred[typ][id] = due
	}
}

func addID(m map[string]map[string]bool, typ, id string) {
	if m[typ] == nil {
		m[typ] = map[string]bool{}
	}
	m[typ][id] = true
}

// RunRound handles the changes noted since the last round. Without rules
// (realtime off or degraded) the changes are dropped: the next reconcile
// after the config comes back catches up.
func (t *Triggers) RunRound(ctx context.Context) (TriggerReport, error) {
	snap, now := t.cfg.Load(), t.clock()
	work, full := t.take(now)
	maps.DeleteFunc(t.lastPost, func(_ string, at time.Time) bool {
		return now.Sub(at) >= realtimelimits.TriggerCooldown
	})
	rules := rulesFor(snap, t.meta)
	if len(rules) == 0 || (len(work) == 0 && len(full) == 0) {
		return TriggerReport{}, nil
	}
	r := t.newRound(ctx, snap, now, rules)
	err := r.run(work, full, false)
	if err != nil {
		t.readFails++
		if t.readFails < realtimelimits.TriggerRetries {
			for typ, ids := range work {
				t.requeue(typ, sortedSet(ids)...)
			}
			t.mu.Lock()
			for typ := range full {
				t.full[typ] = true
			}
			t.mu.Unlock()
		} else {
			slog.ErrorContext(ctx, "realtime: trigger round keeps failing; its changes are left to the next reconcile",
				"error", err)
			t.readFails = 0
		}
		return r.rep, err
	}
	t.readFails = 0
	return r.rep, nil
}

// RunReconcile compares every entity the rules read with what the engine
// has seen and acts on the difference. It catches changes that raised no
// event (a rule on today() that starts to hold, an event the store
// dropped, a write by a process that raises none) and removes the
// seen-sets of rules that are gone. With realtime off or degraded it does
// nothing, so a config that fails to load never erases what was seen.
func (t *Triggers) RunReconcile(ctx context.Context) (TriggerReport, error) {
	snap, now := t.cfg.Load(), t.clock()
	if snap.Config == nil || snap.Degraded {
		return TriggerReport{}, nil
	}
	rules := rulesFor(snap, t.meta)
	r := t.newRound(ctx, snap, now, rules)
	full := map[string]bool{}
	for i := range rules {
		full[rules[i].t.EntityType] = true
	}
	err := r.run(map[string]map[string]bool{}, full, true)
	return r.rep, err
}

// rule is one trigger as a round evaluates it.
type rule struct {
	id    string
	t     realtimecfg.Trigger
	ct    realtimecfg.ChannelType
	mt    *realtimecfg.MessageType
	def   *metamodel.EntityDef
	faces FaceMode
	props map[string]any
}

// rulesFor returns the triggers of snap, sorted by name. It is empty when
// realtime is off or degraded: the engine fails closed.
func rulesFor(snap Snapshot, meta *metamodel.Metamodel) []rule {
	if snap.Config == nil || snap.Degraded {
		return nil
	}
	var out []rule
	for _, tr := range snap.Config.Triggers() {
		ct, ok := snap.Config.ChannelType(tr.Channel)
		if !ok || tr.When.Program == nil {
			continue
		}
		mt := ct.MessageTypes[tr.PostType]
		def, ok := meta.GetEntityDef(tr.EntityType)
		if mt == nil || !ok {
			continue
		}
		faces := FacePrime
		if ct.Anchor == realtimecfg.AnchorFace {
			faces = FaceEach
		}
		props := make(map[string]any, len(tr.Props))
		for k, v := range tr.Props {
			props[k] = v
		}
		out = append(out, rule{id: ruleID(tr, faces), t: tr, ct: ct, mt: mt, def: def, faces: faces, props: props})
	}
	return out
}

// ruleID identifies a trigger's seen-set. It changes with the entity type,
// the rule and the faces read, so an edit that changes what matches starts
// a new seen-set (and a silent baseline) instead of posting a burst.
func ruleID(tr realtimecfg.Trigger, faces FaceMode) string {
	sum := sha256.Sum256([]byte("trigger\x00" + tr.EntityType + "\x00" + tr.When.Source + "\x00" +
		strconv.Itoa(int(faces))))
	return "t-" + tr.Name + "-" + hex.EncodeToString(sum[:4])
}

// ref is the seen-set key of row under r: the id, or id@face for a rule
// that reads each face.
func (r *rule) ref(row EntityRow) string {
	if r.faces == FaceEach {
		return entity.FormatStateRef(row.ID, row.Face)
	}
	return row.ID
}

// key is the channel r posts into for the entity ref.
func (r *rule) key(ref string) channels.Key {
	a := channels.Anchor{ID: ref}
	if r.faces == FaceEach {
		id, face, err := entity.ParseStateRef(ref)
		if err == nil {
			a = channels.Anchor{ID: id, Face: face}
		}
	}
	return channels.Key{Anchor: a, Type: r.ct.Name}
}

// idOf is the entity id of a seen-set key.
func idOf(ref string) string {
	id, _, err := entity.ParseStateRef(ref)
	if err != nil {
		return ref
	}
	return id
}

// eval reports whether row matches r and the fingerprint of what r read.
// ok is false when the answer is unknown: a property r reads cannot be
// read, or the rule fails to evaluate. The seen-set then stays as it was,
// so a broken rule never posts leave.
func (r *rule) eval(ctx context.Context, meta *metamodel.Metamodel, row EntityRow, now time.Time,
) (match bool, fp string, ok bool) {
	for _, f := range row.Inaccessible {
		if slices.Contains(r.t.When.Fields, f) {
			return false, "", false
		}
	}
	b := predicate.NewBindings()
	err := b.SetVar("entity", predicatefns.EntityRecord(meta, r.def, row.ID, r.t.EntityType, row.Props))
	if err == nil {
		err = predicatefns.Bind(b, now)
	}
	var v predicate.Value
	if err == nil {
		v, err = r.t.When.Program.Eval(ctx, b)
	}
	if err != nil {
		slog.DebugContext(ctx, "realtime: trigger rule failed to evaluate; the entity is left as it was",
			"trigger", r.t.Name, "entity", row.ID, "error", err)
		return false, "", false
	}
	got, isBool := v.(predicate.Bool)
	if !isBool {
		return false, "", false
	}
	return got.Bool(), fingerprint(r.t.When.Fields, row.Props), true
}

// round is the state of one round or reconcile: one config snapshot, one
// clock reading, and the shards it has read.
//
//nolint:containedctx // a round lives inside one RunRound or RunReconcile call, which owns ctx
type round struct {
	t      *Triggers
	ctx    context.Context
	snap   Snapshot
	now    time.Time
	rules  []rule
	idx    seenIndex
	shards map[string]*roundShard
	rep    TriggerReport
	// scanned lists the rules whose shards a full scan has read.
	scanned map[string]bool
}

// roundShard is a shard a round has read.
type roundShard struct {
	data    seenShard
	changed bool
	// byID lists the refs of each entity id, for a rule that reads each
	// face; built on first use.
	byID map[string][]string
}

func (t *Triggers) newRound(ctx context.Context, snap Snapshot, now time.Time, rules []rule) *round {
	return &round{t: t, ctx: ctx, snap: snap, now: now, rules: rules, shards: map[string]*roundShard{},
		scanned: map[string]bool{}}
}

// run baselines new rules, adds the candidates of every type in full to
// work, acts on work and saves what changed. A reconcile also removes
// the seen-sets of rules that are gone and splits rules that grew.
func (r *round) run(work map[string]map[string]bool, full map[string]bool, reconcile bool) error {
	idx, err := r.t.seen.loadIndex(r.ctx)
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
	for _, typ := range sortedSet(full) {
		cands, err := r.candidates(typ)
		if err != nil {
			return err
		}
		for id := range cands {
			addID(work, typ, id)
		}
	}
	var queues [][]effect
	for _, typ := range sortedKeys(work) {
		q, err := r.process(typ, sortedSet(work[typ]))
		if err != nil {
			return err
		}
		queues = append(queues, q...)
	}
	r.post(queues)
	r.save()
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
		if _, ok := r.idx.Rules[ru.id]; ok && (typ == "" || ru.t.EntityType == typ) {
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
			k := groupKey{typ: ru.t.EntityType, faces: ru.faces}
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
					if match, fp, ok := ru.eval(r.ctx, r.t.meta, row, r.now); ok && match {
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
			shards := shardsFor(len(entries[i]))
			if err := r.writeLayout(ru.id, shards, entries[i]); err != nil {
				slog.WarnContext(r.ctx, "realtime: trigger baseline not saved; trying again next round",
					"trigger", ru.t.Name, "error", err)
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
	keep := map[string]bool{}
	for i := range r.rules {
		keep[r.rules[i].id] = true
	}
	for id, meta := range r.idx.Rules {
		if !keep[id] {
			gone[id] = meta
			delete(r.idx.Rules, id)
		}
	}
	if len(gone) == 0 {
		return nil
	}
	if err := r.t.seen.saveIndex(r.ctx, r.idx); err != nil {
		return err
	}
	for _, id := range sortedKeys(gone) {
		if err := r.t.seen.dropShards(r.ctx, id, gone[id].Shards); err != nil {
			slog.WarnContext(r.ctx, "realtime: could not delete the seen-set of a removed trigger",
				"rule", id, "error", err)
		}
	}
	return nil
}

// shard returns shard n of ru, read once per round. A shard whose save
// failed in an earlier round is read from memory.
func (r *round) shard(ru *rule, n int) (*roundShard, error) {
	key := shardKey(ru.id, r.idx.Rules[ru.id].Shards, n)
	if sh, ok := r.shards[key]; ok {
		return sh, nil
	}
	if d, ok := r.t.dirty[key]; ok {
		sh := &roundShard{data: d.data, changed: true}
		r.shards[key] = sh
		return sh, nil
	}
	data, err := r.t.seen.loadShard(r.ctx, key)
	if err != nil {
		return nil, err
	}
	sh := &roundShard{data: data}
	r.shards[key] = sh
	return sh, nil
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

// candidates scans every entity of typ and returns the ids whose state
// under some rule differs from what the engine has seen, including ids it
// has seen that no longer exist. It reads rows without acting on them: the
// round reads the candidates again by id and acts on what it reads then.
func (r *round) candidates(typ string) (map[string]bool, error) {
	out := map[string]bool{}
	groups := map[FaceMode][]*rule{}
	for _, ru := range r.active(typ) {
		groups[ru.faces] = append(groups[ru.faces], ru)
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
	present := make([]map[string]bool, len(rules))
	for i, ru := range rules {
		present[i] = map[string]bool{}
		for n := range r.idx.Rules[ru.id].Shards {
			if _, err := r.shard(ru, n); err != nil {
				return err
			}
		}
	}
	err := r.t.rows.Scan(r.ctx, typ, faces, func(rows []EntityRow) error {
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

// differs reports whether row's state under ru differs from what the
// engine has seen. The shard is already read, so this does no I/O.
func (r *round) differs(ru *rule, row EntityRow, ref string) bool {
	sh := r.shards[shardKey(ru.id, r.idx.Rules[ru.id].Shards, shardOf(row.ID, r.idx.Rules[ru.id].Shards))]
	match, fp, ok := ru.eval(r.ctx, r.t.meta, row, r.now)
	prev, had := sh.data[ref]
	return ok && (match != had || (match && fp != prev))
}

// effect is what a rule makes of one entity in a round.
type effect struct {
	ru   *rule
	typ  string
	ref  string
	sh   *roundShard
	on   realtimecfg.TriggerOn
	fp   string
	gone bool // leave: the entry goes
}

func (e effect) apply() {
	if e.gone {
		e.sh.drop(e.ref)
		return
	}
	e.sh.set(e.ref, e.fp)
}

// process reads ids of typ with one batch per face mode, works out each
// active rule's effect on each entity, applies the silent ones and returns
// the rest, one queue per rule.
func (r *round) process(typ string, ids []string) ([][]effect, error) {
	rules := r.active(typ)
	if len(rules) == 0 || len(ids) == 0 {
		return nil, nil
	}
	rows := map[FaceMode]map[string][]EntityRow{}
	for _, ru := range rules {
		if rows[ru.faces] != nil {
			continue
		}
		got, err := r.t.rows.Rows(r.ctx, typ, ids, ru.faces)
		if err != nil {
			return nil, fmt.Errorf("realtime: read %s: %w", typ, err)
		}
		byID := map[string][]EntityRow{}
		for _, row := range got {
			byID[row.ID] = append(byID[row.ID], row)
		}
		rows[ru.faces] = byID
	}
	queues := make([][]effect, len(rules))
	for i, ru := range rules {
		q, err := r.effects(ru, typ, ids, rows[ru.faces])
		if err != nil {
			return nil, err
		}
		queues[i] = q
	}
	return queues, nil
}

// effects works out ru's effect on each of ids, applies the ones that post
// nothing, and returns the ones that post.
func (r *round) effects(ru *rule, typ string, ids []string, rows map[string][]EntityRow) ([]effect, error) {
	var out []effect
	shards := r.idx.Rules[ru.id].Shards
	for _, id := range ids {
		sh, err := r.shard(ru, shardOf(id, shards))
		if err != nil {
			return nil, err
		}
		cur := map[string]EntityRow{}
		for _, row := range rows[id] {
			cur[ru.ref(row)] = row
		}
		refs := sh.refsOf(ru, id)
		for ref := range cur {
			if !slices.Contains(refs, ref) {
				refs = append(refs, ref)
			}
		}
		sort.Strings(refs)
		for _, ref := range refs {
			row, exists := cur[ref]
			if !exists {
				sh.drop(ref) // deleted: no post
				continue
			}
			match, fp, ok := ru.eval(r.ctx, r.t.meta, row, r.now)
			if !ok {
				continue
			}
			prev, had := sh.data[ref]
			e := effect{ru: ru, typ: typ, ref: ref, sh: sh, fp: fp}
			switch {
			case match && !had:
				e.on = realtimecfg.TriggerEnter
			case match && fp != prev:
				e.on = realtimecfg.TriggerChange
			case !match && had:
				e.on, e.gone = realtimecfg.TriggerLeave, true
			default:
				continue
			}
			if e.on != ru.t.On {
				e.apply()
				continue
			}
			out = append(out, e)
		}
	}
	return out, nil
}

// post posts the effects of each rule's queue, taking one from each rule
// in turn until [realtimelimits.MaxSystemPostsPerRound] is spent. An effect left over goes back
// to the next round with its entry untouched, as does one held back by the
// trigger's cooldown, so later changes are seen together with it.
func (r *round) post(queues [][]effect) {
	for {
		progressed := false
		for i := range queues {
			if len(queues[i]) == 0 {
				continue
			}
			e := queues[i][0]
			queues[i] = queues[i][1:]
			progressed = true
			if r.rep.Posted+r.rep.Failed >= realtimelimits.MaxSystemPostsPerRound {
				r.t.requeue(e.typ, idOf(e.ref))
				continue
			}
			r.postOne(e)
		}
		if !progressed {
			return
		}
	}
}

func (r *round) postOne(e effect) {
	k := e.ru.id + "\x00" + e.ref
	if last, ok := r.t.lastPost[k]; ok && r.now.Sub(last) < realtimelimits.TriggerCooldown {
		r.t.postpone(e.typ, idOf(e.ref), last.Add(realtimelimits.TriggerCooldown))
		return
	}
	err := r.t.w.post(r.ctx, r.snap, r.now, e.ru.key(e.ref), e.ru.ct, e.ru.mt, e.ru.props,
		"trigger:"+e.ru.t.Name, e.ru.t.WhenClosed != realtimecfg.WhenClosedPost)
	switch {
	case err == nil:
		r.rep.Posted++
		r.t.lastPost[k] = r.now
	case errors.Is(err, channels.ErrNotOpen):
		r.rep.Skipped++
		slog.DebugContext(r.ctx, "realtime: trigger skipped; the channel is not open",
			"trigger", e.ru.t.Name, "entity", e.ref)
	default:
		r.t.failures[k]++
		if r.t.failures[k] < realtimelimits.TriggerRetries {
			r.t.requeue(e.typ, idOf(e.ref))
			return
		}
		r.rep.Failed++
		slog.ErrorContext(r.ctx, "realtime: trigger post keeps failing; the change is counted as seen",
			"trigger", e.ru.t.Name, "entity", e.ref, "error", err)
	}
	delete(r.t.failures, k)
	e.apply()
}

// save writes every shard the round changed, and every shard an earlier
// round failed to save. A failed save keeps the shard in memory, so this
// process never posts the same change again, and logs once it has failed
// [realtimelimits.TriggerRetries] times.
func (r *round) save() {
	for _, key := range sortedKeys(r.shards) {
		sh := r.shards[key]
		if !sh.changed {
			continue
		}
		r.saveShard(key, sh.data)
	}
	for _, key := range sortedKeys(r.t.dirty) {
		if _, done := r.shards[key]; !done {
			r.saveShard(key, r.t.dirty[key].data)
		}
	}
}

func (r *round) saveShard(key string, data seenShard) {
	err := r.t.seen.saveShard(r.ctx, key, data)
	if err == nil {
		delete(r.t.dirty, key)
		return
	}
	d := r.t.dirty[key]
	if d == nil {
		d = &dirtyShard{}
		r.t.dirty[key] = d
	}
	d.data = data
	d.fails++
	if d.fails == realtimelimits.TriggerRetries {
		slog.ErrorContext(r.ctx, "realtime: the trigger seen-set cannot be saved; this process keeps it in "+
			"memory, and another process may post these changes again", "key", key, "error", err)
	}
}

// reshard splits or joins a rule whose entry count no longer fits its
// shard count. It runs after a reconcile has read and updated every shard. The
// new shards are written beside the old ones and the index switches over,
// so a crash in between leaves the old layout in force.
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
			if !ok || r.t.dirty[key] != nil {
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
			slog.WarnContext(r.ctx, "realtime: could not split a trigger seen-set", "trigger", ru.t.Name, "error", err)
			continue
		}
		r.idx.Rules[ru.id] = seenRule{Shards: shards}
		if err := r.t.seen.saveIndex(r.ctx, r.idx); err != nil {
			r.idx.Rules[ru.id] = meta
			slog.WarnContext(r.ctx, "realtime: could not split a trigger seen-set", "trigger", ru.t.Name, "error", err)
			continue
		}
		if err := r.t.seen.dropShards(r.ctx, ru.id, meta.Shards); err != nil {
			slog.WarnContext(r.ctx, "realtime: could not delete a trigger's old seen-set shards",
				"trigger", ru.t.Name, "error", err)
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].typ != out[j].typ {
			return out[i].typ < out[j].typ
		}
		return out[i].faces < out[j].faces
	})
	return out
}

func sortedSet(m map[string]bool) []string { return sortedKeys(m) }
