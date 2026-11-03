// Package kvpiles is the [piles.Store] backend for the single-process tiers
// (fs, memory, sqlite, desktop), persisting every owner's piles through a
// [state.KV].
//
// # One document, read-modify-write under a lock
//
// All piles live in one JSON value under [StateKey]. Owner strings are inside
// the value and never in a key, so a login name or person id cannot reach a
// file name. Every method takes the store's mutex and then its [Locker],
// re-reads the document, applies its change and writes it back. That makes
// each method atomic: concurrent adds lose nothing and the limits hold.
//
// # Several processes, one machine
//
// The desktop app, `rela mcp`, CLI Lua and `rela scheduler` can all open one
// project, and each builds its own Store over the same cache-directory
// document. The mutex only excludes callers of one instance, so the Locker
// must exclude every other process too: over an on-disk KV that is a
// filelock.Locker on a lock file next to the document. Several MACHINES are out
// of scope; the postgres build uses pgpiles for that.
//
// # Node-local, not in the project database
//
// On the sqlite build the KV handed in is the .rela cache directory, not the
// database's state_kv table. A pile is personal, and rela.db is a file that is
// shipped to other people.
package kvpiles

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// StateKey is where the document lives inside the KV.
const StateKey = "piles.json"

// Store is a [piles.Store] over a [state.KV].
type Store struct {
	kv     state.KV
	locker Locker
	mu     sync.Mutex
}

// Locker excludes every other writer of the document for the span of one
// read-modify-write: other processes over the same KV included. Lock blocks
// until the lock is held or ctx ends, and returns the release func. A KV no
// other process can open takes filelock.ProcessPrivate.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

var _ piles.Store = (*Store)(nil)

// document is the serialized shape.
type document struct {
	Piles []storedPile `json:"piles"`
}

type storedPile struct {
	ID    string `json:"id"`
	Owner string `json:"owner"`
	// Key is set on a declared pile.
	Key     string    `json:"key,omitempty"`
	Name    string    `json:"name"`
	Icon    string    `json:"icon"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	// Items are newest first.
	Items []storedItem `json:"items"`
}

// storedItem keeps id and face apart rather than storing the "ID@face" text
// form, so a document never fails to load over a ref that does not
// re-serialize.
type storedItem struct {
	ID     string     `json:"id"`
	Face   string     `json:"face,omitempty"`
	Added  time.Time  `json:"added"`
	Actor  string     `json:"actor,omitempty"`
	Reason string     `json:"reason,omitempty"`
	ReadAt *time.Time `json:"read_at,omitempty"`
}

func (it storedItem) read() bool { return it.ReadAt != nil }

func (it storedItem) ref() entity.Ref { return entity.Ref{ID: it.ID, Face: entity.Face(it.Face)} }

// New returns a store backed by kv, with locker excluding other processes.
//
// Nil: both rejected. A nil KV would defer the failure to the first pile
// anyone creates, far from the wiring mistake that caused it; a nil locker
// would hide the choice between filelock.Locker and filelock.ProcessPrivate.
func New(kv state.KV, locker Locker) (*Store, error) {
	if kv == nil {
		return nil, errors.New("kvpiles: state KV must be non-nil")
	}
	if locker == nil {
		return nil, errors.New("kvpiles: locker must be non-nil")
	}
	return &Store{kv: kv, locker: locker}, nil
}

// lock takes the mutex and then the locker, and returns the release of both.
func (s *Store) lock(ctx context.Context) (func(), error) {
	s.mu.Lock()
	unlock, err := s.locker.Lock(ctx)
	if err != nil {
		s.mu.Unlock()
		return nil, fmt.Errorf("kvpiles: lock %s: %w", StateKey, err)
	}
	// Returning with s.mu held is the point of lock: the closure unlocks it.
	// nosemgrep: trailofbits.go.missing-unlock-before-return.missing-unlock-before-return
	return func() {
		unlock()
		s.mu.Unlock()
	}, nil
}

// load reads the document. It re-reads on every call, so a change made
// through another handle over the same KV is observed. Caller holds the lock.
//
// A corrupt document is an error, not an empty one. Unlike next-action state,
// piles are things a user collected by hand, and treating a damaged file as
// empty would overwrite it with the next write.
func (s *Store) load(ctx context.Context) (*document, error) {
	data, err := s.kv.Get(ctx, StateKey)
	if os.IsNotExist(err) {
		return &document{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("kvpiles: read %s: %w", StateKey, err)
	}
	var d document
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("kvpiles: %s is not valid JSON (move it aside to start over): %w", StateKey, err)
	}
	return &d, nil
}

// flush writes the document back. Caller holds the lock.
func (s *Store) flush(ctx context.Context, d *document) error {
	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("kvpiles: encode %s: %w", StateKey, err)
	}
	if err := s.kv.Put(ctx, StateKey, data); err != nil {
		return fmt.Errorf("kvpiles: write %s: %w", StateKey, err)
	}
	return nil
}

// read runs fn over the document under the lock. Reads lock too: the KV may
// replace the file while another process writes it.
func (s *Store) read(ctx context.Context, fn func(d *document) error) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	d, err := s.load(ctx)
	if err != nil {
		return err
	}
	return fn(d)
}

// write runs fn over the document under the lock and writes it back when fn
// reports a change.
func (s *Store) write(ctx context.Context, fn func(d *document) (changed bool, err error)) error {
	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	d, err := s.load(ctx)
	if err != nil {
		return err
	}
	changed, err := fn(d)
	if err != nil || !changed {
		return err
	}
	return s.flush(ctx, d)
}

// find returns the index of the owner's pile, or -1. The owner filter is part
// of the lookup, so a foreign pile is indistinguishable from a missing one.
func (d *document) find(owner, id string) int {
	return slices.IndexFunc(d.Piles, func(p storedPile) bool { return p.ID == id && p.Owner == owner })
}

// nameTaken reports whether owner has a pile without a key, other than
// exceptID, named name, compared case-insensitively. Declared piles take no
// part in the name rule.
func (d *document) nameTaken(owner, name, exceptID string) bool {
	return slices.ContainsFunc(d.Piles, func(p storedPile) bool {
		return p.Owner == owner && p.Key == "" && p.ID != exceptID && strings.EqualFold(p.Name, name)
	})
}

func (p storedPile) export() piles.Pile {
	items := make([]piles.Item, len(p.Items))
	for i, it := range p.Items {
		items[i] = piles.Item{Ref: it.ref(), Added: it.Added, Actor: it.Actor, Reason: it.Reason}
		if it.ReadAt != nil {
			items[i].ReadAt = *it.ReadAt
		}
	}
	return piles.Pile{
		ID: p.ID, Owner: p.Owner, Key: p.Key, Name: p.Name, Icon: p.Icon,
		Created: p.Created, Updated: p.Updated, Items: items,
	}
}

// ListPiles returns the owner's piles, oldest first.
func (s *Store) ListPiles(ctx context.Context, owner string) ([]piles.Pile, error) {
	out := []piles.Pile{}
	err := s.read(ctx, func(d *document) error {
		for _, p := range d.Piles {
			if p.Owner == owner {
				out = append(out, p.export())
			}
		}
		return nil
	})
	slices.SortStableFunc(out, func(a, b piles.Pile) int {
		return cmp.Or(a.Created.Compare(b.Created), strings.Compare(a.ID, b.ID))
	})
	return out, err
}

// GetPile returns one of the owner's piles.
func (s *Store) GetPile(ctx context.Context, owner, id string) (piles.Pile, error) {
	var out piles.Pile
	err := s.read(ctx, func(d *document) error {
		i := d.find(owner, id)
		if i < 0 {
			return piles.ErrNotFound
		}
		out = d.Piles[i].export()
		return nil
	})
	return out, err
}

// PileByName finds the owner's pile by case-insensitive name.
func (s *Store) PileByName(ctx context.Context, owner, name string) (piles.Pile, error) {
	var out piles.Pile
	err := s.read(ctx, func(d *document) error {
		i := slices.IndexFunc(d.Piles, func(p storedPile) bool {
			return p.Owner == owner && p.Key == "" && strings.EqualFold(p.Name, name)
		})
		if i < 0 {
			return piles.ErrNotFound
		}
		out = d.Piles[i].export()
		return nil
	})
	return out, err
}

// PileByKey finds the owner's declared pile with that key.
func (s *Store) PileByKey(ctx context.Context, owner, key string) (piles.Pile, error) {
	var out piles.Pile
	err := s.read(ctx, func(d *document) error {
		i := slices.IndexFunc(d.Piles, func(p storedPile) bool { return p.Owner == owner && p.Key == key })
		if key == "" || i < 0 {
			return piles.ErrNotFound
		}
		out = d.Piles[i].export()
		return nil
	})
	return out, err
}

// CreatePile stores p with refs as its first items.
func (s *Store) CreatePile(ctx context.Context, p piles.Pile, refs []entity.Ref, maxPiles, maxItems int) error {
	return s.write(ctx, func(d *document) (bool, error) {
		if slices.ContainsFunc(d.Piles, func(q storedPile) bool { return q.ID == p.ID }) {
			return false, piles.ErrIDTaken
		}
		if p.Key != "" {
			if slices.ContainsFunc(d.Piles, func(q storedPile) bool { return q.Owner == p.Owner && q.Key == p.Key }) {
				return false, piles.ErrKeyTaken
			}
		} else {
			if d.nameTaken(p.Owner, p.Name, "") {
				return false, piles.ErrNameTaken
			}
			count := 0
			for _, q := range d.Piles {
				if q.Owner == p.Owner && q.Key == "" {
					count++
				}
			}
			if count >= maxPiles {
				return false, piles.ErrLimit
			}
		}
		refs = firstUnique(refs, maxItems)
		items := make([]storedItem, len(refs))
		for i, r := range refs {
			items[i] = storedItem{ID: r.ID, Face: string(r.Face), Added: p.Created}
		}
		d.Piles = append(d.Piles, storedPile{
			ID: p.ID, Owner: p.Owner, Key: p.Key, Name: p.Name, Icon: p.Icon,
			Created: p.Created, Updated: p.Updated, Items: items,
		})
		return true, nil
	})
}

// UpdatePile renames or re-icons a pile.
func (s *Store) UpdatePile(ctx context.Context, owner, id, name, icon string, now time.Time) error {
	return s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		if d.nameTaken(owner, name, id) {
			return false, piles.ErrNameTaken
		}
		d.Piles[i].Name, d.Piles[i].Icon, d.Piles[i].Updated = name, icon, now
		return true, nil
	})
}

// DeletePile removes a pile and its items.
func (s *Store) DeletePile(ctx context.Context, owner, id string) error {
	return s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		d.Piles = slices.Delete(d.Piles, i, i+1)
		return true, nil
	})
}

// AddItems puts the new refs on top; with opts.Bump a ref already on the
// pile moves to the top as well. Past the cap it evicts per opts.Overflow
// or, with [piles.KeepExisting], adds only the fresh refs that fit.
func (s *Store) AddItems(
	ctx context.Context, owner, id string, refs []entity.Ref, opts piles.AddOptions,
) (int, error) {
	added := 0
	err := s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		p := &d.Piles[i]
		present := make(map[entity.Ref]bool, len(p.Items))
		for _, it := range p.Items {
			present[it.ref()] = true
		}
		var fresh, bumped []storedItem
		for _, r := range firstUnique(refs, opts.MaxItems) {
			it := storedItem{ID: r.ID, Face: string(r.Face), Added: opts.Now, Actor: opts.Actor, Reason: opts.Reason}
			switch {
			case !present[r]:
				fresh = append(fresh, it)
			case opts.Bump:
				bumped = append(bumped, it)
			}
		}
		if opts.Overflow == piles.KeepExisting {
			fresh = fresh[:min(len(fresh), max(opts.MaxItems-len(p.Items), 0))]
		}
		if len(fresh)+len(bumped) == 0 {
			return false, nil
		}
		added = len(fresh) + len(bumped)
		top := mergeTop(refs, fresh, bumped)
		moved := make(map[entity.Ref]bool, len(bumped))
		for _, it := range bumped {
			moved[it.ref()] = true
		}
		rest := slices.DeleteFunc(p.Items, func(it storedItem) bool { return moved[it.ref()] })
		top = append(top, rest...)
		p.Items = evict(top, opts.MaxItems, opts.Overflow)
		return true, nil
	})
	return added, err
}

// mergeTop orders the fresh and bumped items as their refs came in, the
// first newest.
func mergeTop(refs []entity.Ref, fresh, bumped []storedItem) []storedItem {
	byRef := make(map[entity.Ref]storedItem, len(fresh)+len(bumped))
	for _, it := range fresh {
		byRef[it.ref()] = it
	}
	for _, it := range bumped {
		byRef[it.ref()] = it
	}
	out := make([]storedItem, 0, len(byRef))
	for _, r := range refs {
		if it, ok := byRef[r]; ok {
			out = append(out, it)
			delete(byRef, r)
		}
	}
	return out
}

// evict cuts items, newest first, to limit. [piles.EvictReadFirst] drops
// read items before unread ones; every other overflow drops the oldest. The
// survivors keep their order.
func evict(items []storedItem, limit int, overflow piles.Overflow) []storedItem {
	if len(items) <= limit {
		return items
	}
	if overflow != piles.EvictReadFirst {
		return items[:limit]
	}
	keep := make(map[int]bool, limit)
	for i, it := range items {
		if len(keep) < limit && !it.read() {
			keep[i] = true
		}
	}
	for i := range items {
		if len(keep) < limit {
			keep[i] = true
		}
	}
	out := make([]storedItem, 0, limit)
	for i, it := range items {
		if keep[i] {
			out = append(out, it)
		}
	}
	return out
}

// RemoveItems drops refs from the pile, ignoring refs not on it.
func (s *Store) RemoveItems(ctx context.Context, owner, id string, refs []entity.Ref) error {
	drop := make(map[entity.Ref]bool, len(refs))
	for _, r := range refs {
		drop[r] = true
	}
	return s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		before := len(d.Piles[i].Items)
		d.Piles[i].Items = slices.DeleteFunc(d.Piles[i].Items, func(it storedItem) bool { return drop[it.ref()] })
		return len(d.Piles[i].Items) != before, nil
	})
}

// MarkRead stamps unread items naming refs, or all unread items, with now.
func (s *Store) MarkRead(ctx context.Context, owner, id string, refs []entity.Ref, all bool, now time.Time) error {
	want := make(map[entity.Ref]bool, len(refs))
	for _, r := range refs {
		want[r] = true
	}
	return s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		changed := false
		for j := range d.Piles[i].Items {
			it := &d.Piles[i].Items[j]
			if !it.read() && (all || want[it.ref()]) {
				stamp := now
				it.ReadAt = &stamp
				changed = true
			}
		}
		return changed, nil
	})
}

// PruneItems drops items read before readBefore or added before addedBefore.
func (s *Store) PruneItems(ctx context.Context, owner, id string, readBefore, addedBefore time.Time) error {
	return s.write(ctx, func(d *document) (bool, error) {
		i := d.find(owner, id)
		if i < 0 {
			return false, piles.ErrNotFound
		}
		before := len(d.Piles[i].Items)
		d.Piles[i].Items = slices.DeleteFunc(d.Piles[i].Items, func(it storedItem) bool {
			return (!readBefore.IsZero() && it.ReadAt != nil && it.ReadAt.Before(readBefore)) ||
				(!addedBefore.IsZero() && it.Added.Before(addedBefore))
		})
		return len(d.Piles[i].Items) != before, nil
	})
}

// RenameEntity rewrites items naming oldID. Where the new ref is already on
// a pile, the existing item keeps its place and the renamed one is dropped.
func (s *Store) RenameEntity(ctx context.Context, oldID, newID string) error {
	if oldID == newID {
		return nil
	}
	return s.write(ctx, func(d *document) (bool, error) {
		changed := false
		for pi := range d.Piles {
			if renameItems(&d.Piles[pi], oldID, newID) {
				changed = true
			}
		}
		return changed, nil
	})
}

// RenameOwner moves oldOwner's piles to newOwner, and the actor of every
// item oldOwner caused.
//
// It runs for a renamed person, whose new id is fresh, so newOwner normally
// holds no piles. If it does, they can only be a leftover the delete hook
// missed: where one clashes by name with a moved pile, the moved pile wins and
// the leftover is dropped, so the one-name-per-owner rule still holds.
func (s *Store) RenameOwner(ctx context.Context, oldOwner, newOwner string) error {
	if oldOwner == newOwner {
		return nil
	}
	return s.write(ctx, func(d *document) (bool, error) {
		movedNames, movedKeys := map[string]bool{}, map[string]bool{}
		moving := renameActor(d, oldOwner, newOwner)
		for _, p := range d.Piles {
			if p.Owner != oldOwner {
				continue
			}
			moving = true
			if p.Key != "" {
				movedKeys[p.Key] = true
			} else {
				movedNames[strings.ToLower(p.Name)] = true
			}
		}
		if !moving {
			return false, nil
		}
		d.Piles = slices.DeleteFunc(d.Piles, func(p storedPile) bool {
			if p.Owner != newOwner {
				return false
			}
			if p.Key != "" {
				return movedKeys[p.Key]
			}
			return movedNames[strings.ToLower(p.Name)]
		})
		for pi := range d.Piles {
			if d.Piles[pi].Owner == oldOwner {
				d.Piles[pi].Owner = newOwner
			}
		}
		return true, nil
	})
}

// renameActor rewrites the actor on every item oldOwner caused, in anyone's
// pile, and reports whether any changed.
func renameActor(d *document, oldOwner, newOwner string) bool {
	changed := false
	for pi := range d.Piles {
		for ii := range d.Piles[pi].Items {
			if d.Piles[pi].Items[ii].Actor == oldOwner {
				d.Piles[pi].Items[ii].Actor = newOwner
				changed = true
			}
		}
	}
	return changed
}

// DeleteOwner drops every pile owned by owner.
func (s *Store) DeleteOwner(ctx context.Context, owner string) error {
	return s.write(ctx, func(d *document) (bool, error) {
		before := len(d.Piles)
		d.Piles = slices.DeleteFunc(d.Piles, func(p storedPile) bool { return p.Owner == owner })
		return len(d.Piles) != before, nil
	})
}

// renameItems rewrites one pile's items and reports whether any changed.
func renameItems(p *storedPile, oldID, newID string) bool {
	if !slices.ContainsFunc(p.Items, func(it storedItem) bool { return it.ID == oldID }) {
		return false
	}
	taken := map[string]bool{}
	for _, it := range p.Items {
		if it.ID == newID {
			taken[it.Face] = true
		}
	}
	out := p.Items[:0]
	for _, it := range p.Items {
		if it.ID == oldID {
			if taken[it.Face] {
				continue
			}
			it.ID = newID
		}
		out = append(out, it)
	}
	p.Items = out
	return true
}

// DeleteEntity drops items naming id.
func (s *Store) DeleteEntity(ctx context.Context, id string) error {
	return s.write(ctx, func(d *document) (bool, error) {
		changed := false
		for pi := range d.Piles {
			n := len(d.Piles[pi].Items)
			d.Piles[pi].Items = slices.DeleteFunc(d.Piles[pi].Items, func(it storedItem) bool { return it.ID == id })
			changed = changed || len(d.Piles[pi].Items) != n
		}
		return changed, nil
	})
}

// DeleteFace drops items naming exactly that face of id.
func (s *Store) DeleteFace(ctx context.Context, id string, face entity.Face) error {
	return s.write(ctx, func(d *document) (bool, error) {
		changed := false
		for pi := range d.Piles {
			n := len(d.Piles[pi].Items)
			d.Piles[pi].Items = slices.DeleteFunc(d.Piles[pi].Items, func(it storedItem) bool {
				return it.ID == id && it.Face == string(face)
			})
			changed = changed || len(d.Piles[pi].Items) != n
		}
		return changed, nil
	})
}

// firstUnique collapses duplicate refs, keeping the first, and keeps at most
// limit of them.
func firstUnique(refs []entity.Ref, limit int) []entity.Ref {
	seen := make(map[entity.Ref]bool, len(refs))
	out := make([]entity.Ref, 0, min(len(refs), limit))
	for _, r := range refs {
		if len(out) == limit {
			break
		}
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}
