// Package listorder holds each user's manual order of a list's rows
// (TKT-AVNEX5T).
//
// # Why this is not in the graph
//
// A list with `order: manual` lets every user drag its rows into their own
// order. That order is a fact about one person's work, not about the
// entities, so it is kept outside store.Store, like internal/piles and
// internal/userstate. Membership stays computed by the list; only the order
// of the rows a user has placed is stored.
//
// # Privacy
//
// The owner is always the stamped principal. No call reads or writes
// another user's order. Stored ids never leave the server: [Apply] only
// permutes rows the caller already passed through the row gate, so a hidden
// row takes no slot and its id is not disclosed.
//
// # Storage
//
// One JSON document per (owner, list) on a [state.KV], under a key built
// from hashes, so neither a user name nor a list id shapes a path. A
// document holds at most [MaxIDs] ids. Writes for one key are serialized in
// process; across nodes the last writer wins, which is acceptable because
// only the owner writes their own order.
package listorder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

const (
	// MaxIDs is the number of positioned rows kept per user and list.
	MaxIDs = 500
	// MaxIDBytes bounds one stored id.
	MaxIDBytes = 200
)

var (
	// ErrNoOwner reports a call without a usable owner: no principal, the
	// unknown principal, or a reserved system identity.
	ErrNoOwner = errors.New("listorder: no owner identity")
	// ErrNotFound reports a move whose row or neighbor is not among the
	// rows of the list as the caller sees them.
	ErrNotFound = errors.New("listorder: row not in list")
	// ErrBadMove reports a move that names no neighbor, both, or the row
	// itself.
	ErrBadMove = errors.New("listorder: a move names exactly one other row as before or after")
)

// Service reads and writes manual orders.
type Service struct {
	kv state.KV
	// locks serialize writes per key in a fixed set of stripes, so the set
	// does not grow with users and lists.
	locks [lockStripes]sync.Mutex
}

const lockStripes = 64

// New returns a service over kv.
//
// Nil: rejected — a service without storage would forget every move.
func New(kv state.KV) (*Service, error) {
	if kv == nil {
		return nil, errors.New("listorder: nil state KV")
	}
	return &Service{kv: kv}, nil
}

type document struct {
	IDs []string `json:"ids"`
}

// ownerFrom returns the acting user's owner key.
func ownerFrom(ctx context.Context) (string, error) {
	p, ok := principal.Stamped(ctx)
	if !ok {
		return "", ErrNoOwner
	}
	user := strings.TrimSpace(p.User)
	if user == "" || user == principal.Unknown || principal.IsReserved(user) {
		return "", ErrNoOwner
	}
	return user, nil
}

// HasOwner reports whether ctx carries a user who can hold an order.
func HasOwner(ctx context.Context) bool {
	_, err := ownerFrom(ctx)
	return err == nil
}

func hashed(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

func keyFor(owner, list string) string {
	return "listorder/" + hashed(owner) + "/" + hashed(list)
}

// Order returns the caller's stored order of list, first to last. A missing
// document is an empty order. An unreadable one is an empty order too, and
// is logged: losing an order costs a re-drag, not data.
func (s *Service) Order(ctx context.Context, list string) ([]string, error) {
	owner, err := ownerFrom(ctx)
	if err != nil {
		return nil, err
	}
	return s.read(ctx, keyFor(owner, list))
}

func (s *Service) read(ctx context.Context, key string) ([]string, error) {
	data, err := s.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listorder: read: %w", err)
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		slog.WarnContext(ctx, "listorder: unreadable order, starting empty", "err", err)
		return nil, nil
	}
	return clean(doc.IDs), nil
}

// Move places id next to a neighbor and stores the result. full is the
// list's rows as the caller sees them now, with the stored order already
// applied (see [Apply]). Exactly one of before and after names the
// neighbor; the row and the neighbor must both be in full, or the move
// fails with [ErrNotFound]. See [Placed] for what is stored.
//
// The stored order is read and written under one lock, so two moves of
// one user in one process do not lose each other. Across nodes the last
// writer wins.
func (s *Service) Move(ctx context.Context, list string, full []string, id, before, after string) error {
	owner, err := ownerFrom(ctx)
	if err != nil {
		return err
	}
	key := keyFor(owner, list)
	lock := s.lockFor(key)
	lock.Lock()
	defer lock.Unlock()
	old, err := s.read(ctx, key)
	if err != nil {
		return err
	}
	next, err := Placed(full, old, id, before, after)
	if err != nil {
		return err
	}
	data, err := json.Marshal(document{IDs: next})
	if err != nil {
		return fmt.Errorf("listorder: encode: %w", err)
	}
	if err := s.kv.Put(ctx, key, data); err != nil {
		return fmt.Errorf("listorder: write: %w", err)
	}
	return nil
}

func (s *Service) lockFor(key string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return &s.locks[h.Sum32()%lockStripes]
}

// Placed returns the order to store after moving id next to its neighbor.
//
// full is the list as the caller sees it, old the stored order. The result
// holds the rows of full, with id moved, from the top through the moved row
// or the last row of old, whichever is lower, so no earlier placement is
// lost. Rows below keep the list sort. Ids of old that are not in full (a
// row filtered out, hidden, or deleted) keep their place after the stored
// id they followed; they take no slot when the list is shown. At most
// [MaxIDs] ids are kept: the moved row always, and the lowest others are
// dropped first.
func Placed(full, old []string, id, before, after string) ([]string, error) {
	if (before == "") == (after == "") || id == "" || id == before || id == after {
		return nil, ErrBadMove
	}
	from := slices.Index(full, id)
	neighbor := before + after
	if from < 0 || !slices.Contains(full, neighbor) {
		return nil, ErrNotFound
	}
	rest := slices.Delete(slices.Clone(full), from, from+1)
	at := slices.Index(rest, neighbor)
	if after != "" {
		at++
	}
	moved := slices.Insert(rest, at, id)
	end := at
	for _, o := range old {
		end = max(end, slices.Index(moved, o))
	}
	out := slices.Clone(moved[:end+1])
	prev := -1
	for _, o := range old {
		if i := slices.Index(out, o); i >= 0 {
			prev = i
			continue
		}
		prev++
		out = slices.Insert(out, prev, o)
	}
	out = clean(out)
	if len(out) == MaxIDs && !slices.Contains(out, id) {
		out = append(out[:MaxIDs-1], id)
	}
	return out, nil
}

// Apply returns the order in which to show rows: the indexes of rows whose
// id is in ids first, in stored order, then the others in their incoming
// order. The result is a permutation of 0..len(rows)-1.
func Apply(rows, ids []string) []int {
	rank := make(map[string]int, len(ids))
	for i, id := range ids {
		if _, dup := rank[id]; !dup {
			rank[id] = i
		}
	}
	placed := make([]int, 0, len(ids))
	rest := make([]int, 0, len(rows))
	for i, id := range rows {
		if _, ok := rank[id]; ok {
			placed = append(placed, i)
		} else {
			rest = append(rest, i)
		}
	}
	slices.SortStableFunc(placed, func(a, b int) int { return rank[rows[a]] - rank[rows[b]] })
	return append(placed, rest...)
}

// clean drops empty, oversized, control-character and duplicate ids and
// caps the list at [MaxIDs].
func clean(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, min(len(ids), MaxIDs))
	for _, id := range ids {
		if len(out) == MaxIDs {
			break
		}
		if id == "" || len(id) > MaxIDBytes || seen[id] || strings.ContainsFunc(id, isControl) {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }
