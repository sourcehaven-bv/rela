// Package presence tracks who has an entity open (TKT-MHXH87).
//
// A [Tracker] is in memory and passive: it starts no goroutine and reads no
// clock. Every call that depends on time takes now from the caller, so the
// live worker drives it from its fixed ticker and tests drive it from a fake
// clock. Nothing is persisted; a new Tracker starts empty.
//
// A viewer is one entry per (key, identity). The key is the resolved entity
// face the caller chose; the tracker does not interpret it. Each entry counts
// the slots holding it open, so several tabs of one identity are one entry.
// When the last slot leaves with grace, the entry stays for [Grace] and a
// rejoin within that time keeps its start time.
//
// The tracker never calls out while holding its lock, so a caller may hold
// its own locks around every method.
package presence

import (
	"cmp"
	"log/slog"
	"slices"
	"sync"
	"time"
)

const (
	// Grace is how long an entry stays after its last slot left with grace.
	Grace = 15 * time.Second
	// MaxPerIdentity bounds the live and in-grace entries of one identity.
	MaxPerIdentity = 64
	// MaxEntries bounds the live and in-grace entries of the tracker.
	MaxEntries = 4096
)

// Identity is who a viewer is: the fields principal.Principal.SameIdentity
// compares.
type Identity struct {
	User    string
	Tool    string
	RawUser string
	OrgID   string
}

// Slot is one holder of an entry: a subscription Sub on the stream Conn.
type Slot struct {
	Conn string
	Sub  string
}

// Viewer is an entry as [Tracker.Viewers] reports it. Since is when the
// entry was created; a rejoin within grace keeps it.
type Viewer struct {
	Identity
	Since time.Time
}

type entry struct {
	since time.Time
	slots map[Slot]struct{}
	// until is when an entry without slots expires; zero while a slot holds
	// it.
	until time.Time
}

type entryRef struct {
	key string
	who Identity
}

// Tracker is the presence state. The zero value is not usable; call [New].
type Tracker struct {
	mu    sync.Mutex
	byKey map[string]map[Identity]*entry
	// slots is the entry each slot holds, indexed by connection, so
	// LeaveConn touches only that connection's slots.
	slots       map[string]map[string]entryRef
	perIdentity map[Identity]int
	total       int
	changed     map[string]struct{}
	// published is the viewers per key as of the last TakeChanged. Viewers
	// reads it, so expiry takes effect on the caller's tick only.
	published map[string][]Viewer
	logged    bool // the global cap was hit and logged
}

// New returns an empty tracker.
func New() *Tracker {
	return &Tracker{
		byKey:       make(map[string]map[Identity]*entry),
		slots:       make(map[string]map[string]entryRef),
		perIdentity: make(map[Identity]int),
		changed:     make(map[string]struct{}),
		published:   make(map[string][]Viewer),
	}
}

// Join records that slot holds key open as who. A slot holds one key at a
// time: when it held another, that one is left without grace first. Joining
// an entry that is in grace keeps its start time.
//
// It reports false, recording nothing, when who already has
// [MaxPerIdentity] entries or the tracker has [MaxEntries]. The identity
// cap is checked first, so one identity cannot fill the tracker for others.
func (t *Tracker) Join(key string, slot Slot, who Identity, now time.Time) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	ref := entryRef{key: key, who: who}
	if cur, ok := t.slots[slot.Conn][slot.Sub]; ok {
		if cur == ref {
			return true
		}
		t.detachLocked(slot, cur, now, false)
	}
	if e := t.byKey[key][who]; e != nil {
		e.slots[slot] = struct{}{}
		e.until = time.Time{}
		t.setSlotLocked(slot, ref)
		return true
	}
	if t.perIdentity[who] >= MaxPerIdentity {
		return false
	}
	if t.total >= MaxEntries {
		if !t.logged {
			t.logged = true
			slog.Warn("presence: tracker full; further viewers are not shown", "entries", t.total)
		}
		return false
	}
	byWho := t.byKey[key]
	if byWho == nil {
		byWho = make(map[Identity]*entry)
		t.byKey[key] = byWho
	}
	byWho[who] = &entry{since: now, slots: map[Slot]struct{}{slot: {}}}
	t.setSlotLocked(slot, ref)
	t.perIdentity[who]++
	t.total++
	t.changed[key] = struct{}{}
	return true
}

// Leave records that slot no longer holds its key open. With grace, an
// entry left without slots stays for [Grace]; without, it goes at once. A
// slot that holds nothing is ignored.
func (t *Tracker) Leave(slot Slot, now time.Time, grace bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if ref, ok := t.slots[slot.Conn][slot.Sub]; ok {
		t.detachLocked(slot, ref, now, grace)
	}
}

// LeaveConn leaves, with grace, every slot of the stream conn. Its cost is
// proportional to that stream's slots.
func (t *Tracker) LeaveConn(conn string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for sub, ref := range t.slots[conn] {
		t.detachLocked(Slot{Conn: conn, Sub: sub}, ref, now, true)
	}
}

func (t *Tracker) setSlotLocked(slot Slot, ref entryRef) {
	bySub := t.slots[slot.Conn]
	if bySub == nil {
		bySub = make(map[string]entryRef)
		t.slots[slot.Conn] = bySub
	}
	bySub[slot.Sub] = ref
}

func (t *Tracker) detachLocked(slot Slot, ref entryRef, now time.Time, grace bool) {
	if bySub := t.slots[slot.Conn]; bySub != nil {
		delete(bySub, slot.Sub)
		if len(bySub) == 0 {
			delete(t.slots, slot.Conn)
		}
	}
	e := t.byKey[ref.key][ref.who]
	if e == nil {
		return
	}
	delete(e.slots, slot)
	if len(e.slots) > 0 {
		return
	}
	if grace {
		e.until = now.Add(Grace)
		return
	}
	t.removeLocked(ref)
}

func (t *Tracker) removeLocked(ref entryRef) {
	byWho := t.byKey[ref.key]
	delete(byWho, ref.who)
	if len(byWho) == 0 {
		delete(t.byKey, ref.key)
	}
	t.perIdentity[ref.who]--
	if t.perIdentity[ref.who] <= 0 {
		delete(t.perIdentity, ref.who)
	}
	t.total--
	t.changed[ref.key] = struct{}{}
}

// TakeChanged removes the entries whose grace ended by now, publishes the
// viewers of every key that changed since the last call, and returns those
// keys sorted.
func (t *Tracker) TakeChanged(now time.Time) []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	for key, byWho := range t.byKey {
		for who, e := range byWho {
			if len(e.slots) == 0 && !now.Before(e.until) {
				t.removeLocked(entryRef{key: key, who: who})
			}
		}
	}
	keys := make([]string, 0, len(t.changed))
	for key := range t.changed {
		keys = append(keys, key)
		t.publishLocked(key)
	}
	clear(t.changed)
	slices.Sort(keys)
	return keys
}

func (t *Tracker) publishLocked(key string) {
	byWho := t.byKey[key]
	if len(byWho) == 0 {
		delete(t.published, key)
		return
	}
	vs := make([]Viewer, 0, len(byWho))
	for who, e := range byWho {
		vs = append(vs, Viewer{Identity: who, Since: e.since})
	}
	slices.SortFunc(vs, func(a, b Viewer) int {
		return cmp.Or(a.Since.Compare(b.Since), cmp.Compare(a.User, b.User), cmp.Compare(a.Tool, b.Tool),
			cmp.Compare(a.RawUser, b.RawUser), cmp.Compare(a.OrgID, b.OrgID))
	})
	t.published[key] = vs
}

// Viewers returns the viewers of key as of the last [Tracker.TakeChanged],
// oldest first, then by user. The caller owns the returned slice.
func (t *Tracker) Viewers(key string) []Viewer {
	t.mu.Lock()
	defer t.mu.Unlock()
	return slices.Clone(t.published[key])
}

// Len returns the number of live and in-grace entries.
func (t *Tracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.total
}

// Count returns the number of live and in-grace entries of who.
func (t *Tracker) Count(who Identity) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.perIdentity[who]
}
