package realtime

import (
	"cmp"
	"container/heap"
	"slices"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
)

// typingEntry is one identity's typing indicator in one channel. since is
// when the entry started and does not move on a refresh; deadline is when
// the server drops it unless the identity refreshes it first.
type typingEntry struct {
	author   channels.Author
	since    time.Time
	deadline time.Time
}

// liveTyping is a live typing entry and the identity that holds it.
type liveTyping struct {
	identity string
	typingEntry
}

// typingStore holds the typing entries of every channel in this process:
// per channel key, one entry per identity. It is apart from the
// [ephemeralStore] ring, so typing and operator signals never take each
// other's slots or evict each other.
//
// Both caps refuse with a 429 and never evict: perChannel entries in one
// channel, and channels holding entries in the process. A refresh of a held
// entry passes either cap.
//
// No call on the request path walks the whole store. touch, clear and live
// visit only the channel they name. Expiry runs off a min-heap of
// deadlines: sweep pops only what has expired, and nextDeadline peeks at
// the top. A refresh or a clear leaves its old heap item behind; sweep
// skips an item whose entry is gone or holds another deadline, and an
// early timer fire on such an item finds nothing to drop.
//
// Entries are never stored and never audited, and the store is process
// local, as signals are.
type typingStore struct {
	mu         sync.Mutex
	perChannel int
	channels   int
	byKey      map[channels.Key]map[string]typingEntry
	deadlines  deadlineHeap
	// visits counts the entries the store looked at, so a test can pin
	// that a call stays within its channel.
	visits int
}

func newTypingStore(perChannel, maxChannels int) *typingStore {
	return &typingStore{
		perChannel: perChannel, channels: maxChannels,
		byKey: map[channels.Key]map[string]typingEntry{},
	}
}

func defaultTypingStore() *typingStore {
	return newTypingStore(realtimelimits.MaxTypingPerChannel, realtimelimits.MaxTypingChannels)
}

// admits reports whether touch would take identity's entry in key at now
// without a cap refusal, so the caller can check before it charges a rate
// token. It visits only key's entries.
func (t *typingStore) admits(key channels.Key, identity string, now time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.roomLocked(key, identity, now)
}

// roomLocked is the cap check of admits and touch. Expired entries of key
// count as absent. A channel that holds only expired entries still counts
// toward the channel cap until the expiry sweep drops it, so the channel
// cap's Retry-After is the TTL: by then every entry held now has expired
// or been refreshed.
func (t *typingStore) roomLocked(key channels.Key, identity string, now time.Time) error {
	entries := t.byKey[key]
	if e, ok := entries[identity]; ok && now.Before(e.deadline) {
		return nil
	}
	live, soonest := 0, time.Time{}
	for _, e := range entries {
		t.visits++
		if now.Before(e.deadline) {
			live++
			if soonest.IsZero() || e.deadline.Before(soonest) {
				soonest = e.deadline
			}
		}
	}
	switch {
	case live >= t.perChannel:
		return rateLimited(soonest.Sub(now))
	case entries == nil && len(t.byKey) >= t.channels:
		return rateLimited(realtimelimits.TypingTTL)
	}
	return nil
}

// touch starts or refreshes identity's entry in key at now. It drops the
// expired entries of key first and reports whether it dropped any, for the
// caller to mark key. A held entry gets a new deadline and keeps its since
// (started is false); a new entry is checked against both caps.
func (t *typingStore) touch(
	key channels.Key, identity string, author channels.Author, now time.Time,
) (started, pruned bool, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	pruned = t.pruneLocked(key, now)
	deadline := now.Add(realtimelimits.TypingTTL)
	entries := t.byKey[key]
	e, held := entries[identity]
	if !held {
		if err := t.roomLocked(key, identity, now); err != nil {
			return false, pruned, err
		}
		if entries == nil {
			entries = map[string]typingEntry{}
			t.byKey[key] = entries
		}
		e = typingEntry{author: author, since: now}
	}
	e.deadline = deadline
	entries[identity] = e
	heap.Push(&t.deadlines, deadlineItem{at: deadline, key: key, identity: identity})
	return !held, pruned, nil
}

// pruneLocked drops the expired entries of key and reports whether it
// dropped any.
func (t *typingStore) pruneLocked(key channels.Key, now time.Time) bool {
	entries := t.byKey[key]
	n := len(entries)
	for id, e := range entries {
		t.visits++
		if !now.Before(e.deadline) {
			delete(entries, id)
		}
	}
	if entries != nil && len(entries) == 0 {
		delete(t.byKey, key)
	}
	return len(entries) != n
}

// clear drops identity's entry in key and reports whether it held one.
func (t *typingStore) clear(key channels.Key, identity string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	entries := t.byKey[key]
	if _, ok := entries[identity]; !ok {
		return false
	}
	delete(entries, identity)
	if len(entries) == 0 {
		delete(t.byKey, key)
	}
	return true
}

// live returns the entries of key that are live at now, sorted by since,
// then by author user.
func (t *typingStore) live(key channels.Key, now time.Time) []liveTyping {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []liveTyping
	for id, e := range t.byKey[key] {
		t.visits++
		if now.Before(e.deadline) {
			out = append(out, liveTyping{identity: id, typingEntry: e})
		}
	}
	slices.SortFunc(out, func(a, b liveTyping) int {
		if c := a.since.Compare(b.since); c != 0 {
			return c
		}
		if c := cmp.Compare(a.author.User, b.author.User); c != 0 {
			return c
		}
		return cmp.Compare(a.identity, b.identity)
	})
	return out
}

// sweep drops the entries whose deadline passed by now and returns the
// keys that changed. It pops only due heap items, so it returns at once
// when nothing is due.
func (t *typingStore) sweep(now time.Time) []channels.Key {
	t.mu.Lock()
	defer t.mu.Unlock()
	seen := map[channels.Key]bool{}
	var changed []channels.Key
	for len(t.deadlines) > 0 && !now.Before(t.deadlines[0].at) {
		it, _ := heap.Pop(&t.deadlines).(deadlineItem) // only deadlineItems are pushed
		entries := t.byKey[it.key]
		e, ok := entries[it.identity]
		if !ok || !e.deadline.Equal(it.at) {
			continue // refreshed or cleared since this item was pushed
		}
		delete(entries, it.identity)
		if len(entries) == 0 {
			delete(t.byKey, it.key)
		}
		if !seen[it.key] {
			seen[it.key] = true
			changed = append(changed, it.key)
		}
	}
	channels.SortKeys(changed)
	return changed
}

// nextDeadline returns the earliest pending deadline. It may belong to an
// entry that was refreshed or cleared since; the timer then fires early
// and finds nothing to drop.
func (t *typingStore) nextDeadline() (time.Time, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.deadlines) == 0 {
		return time.Time{}, false
	}
	return t.deadlines[0].at, true
}

// dropAnchor drops the entries of every channel of entity id, or only those
// anchored to face when face is non-empty, and returns their keys. It runs
// on an entity delete or rename, not on the request path.
func (t *typingStore) dropAnchor(id string, face entity.Face) []channels.Key {
	t.mu.Lock()
	defer t.mu.Unlock()
	var dropped []channels.Key
	for key := range t.byKey {
		if key.ID != id || (face != "" && key.Face != face) {
			continue
		}
		delete(t.byKey, key)
		dropped = append(dropped, key)
	}
	channels.SortKeys(dropped)
	return dropped
}

// deadlineItem is one pushed deadline of an entry.
type deadlineItem struct {
	at       time.Time
	key      channels.Key
	identity string
}

// deadlineHeap is a min-heap of deadlines for container/heap.
type deadlineHeap []deadlineItem

func (h deadlineHeap) Len() int           { return len(h) }
func (h deadlineHeap) Less(i, j int) bool { return h[i].at.Before(h[j].at) }
func (h deadlineHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *deadlineHeap) Push(x any) {
	if it, ok := x.(deadlineItem); ok {
		*h = append(*h, it)
	}
}

func (h *deadlineHeap) Pop() any {
	old := *h
	it := old[len(old)-1]
	*h = old[:len(old)-1]
	return it
}
