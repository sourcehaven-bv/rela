// Package piles holds per-user piles: named, personal stacks of entity
// references that a user fills from lists, search, scripts and automations,
// and then steps through, acts on or exports.
//
// # Why this is not in the graph
//
// A pile is a fact about one person's work, not about the entities on it.
// Storing it as an entity would make it visible to everyone, audited in the
// append-only log, versioned, and run through automations. A collection a
// team should share is graph data: an entity with relations. So piles are a
// separate service with their own backends, like internal/userstate and
// internal/comments, deliberately outside store.Store.
//
// # Privacy
//
// Only the owner reads a pile. Lua, automations and MCP may PUSH into
// another user's pile (see [Service.Push]), but nothing reads one on another
// user's behalf. A pile stores references only; every read path resolves
// them through the caller's visibility gate, so a pile never shows an entity
// its owner cannot read.
//
// # Ordering and limits
//
// A pile is a stack: items come back newest first. A pile holds at most
// [MaxItems] items; the owner's own add past that evicts the oldest in the
// same atomic step, so adding never fails on size. A push into another user's
// pile never evicts (see [Overflow]). A user holds at most [MaxPiles] piles,
// of which pushes by others may create at most [MaxForeignPiles].
//
// # Backends
//
// Implementations live in subpackages and are selected at wiring time
// (internal/appbuild). Every implementation must pass pilestest.RunAll.
package piles

import (
	"context"
	"errors"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

const (
	// MaxPiles is the number of piles one owner may hold.
	MaxPiles = 50
	// MaxForeignPiles is the number of piles an owner may hold for a push by
	// another user to still create one. Past it such a push is a silent
	// no-op, so the owner always keeps MaxPiles-MaxForeignPiles slots that
	// only they can fill: others cannot use up an owner's pile cap.
	MaxForeignPiles = 25
	// MaxItems is the number of items one pile holds. Adding past it evicts
	// the oldest items.
	MaxItems = 500
	// MaxNameRunes bounds a pile name.
	MaxNameRunes = 80
	// DefaultIcon is the icon of a pile created without one.
	DefaultIcon = "layers"
)

var (
	// ErrNotFound reports a pile that does not exist or belongs to another
	// owner. The two are deliberately one error: a pile id is not a secret
	// worth an oracle, but there is no reason to offer one either.
	ErrNotFound = errors.New("piles: pile not found")
	// ErrNoOwner reports a call without a usable owner: no principal, an
	// unknown principal, or a reserved system identity.
	ErrNoOwner = errors.New("piles: no owner identity")
	// ErrUnknownOwner reports a push to an owner that is neither the acting
	// user nor an existing person entity.
	ErrUnknownOwner = errors.New("piles: unknown owner")
	// ErrLimit reports an owner at [MaxPiles].
	ErrLimit = errors.New("piles: pile limit reached")
	// ErrNameTaken reports a pile name the owner already uses (compared
	// case-insensitively).
	ErrNameTaken = errors.New("piles: pile name already in use")
	// ErrInvalid reports invalid input; the wrapped message says which.
	ErrInvalid = errors.New("piles: invalid input")
	// ErrIDTaken reports a pile id that already exists. Ids are minted at
	// random, so this is a rare clash the service retries with a fresh id.
	ErrIDTaken = errors.New("piles: pile id already in use")
)

// Overflow says what [Store.AddItems] does when the fresh refs do not fit
// under the item cap.
type Overflow int

const (
	// KeepExisting never removes an item. Only as many fresh refs as there
	// is room for are added, the first refs first; the rest are dropped. A
	// push into another user's pile uses it, so a pusher can never displace
	// what the owner collected. It is the zero value, so a forgotten choice
	// fails toward keeping data.
	KeepExisting Overflow = iota
	// EvictOldest adds every fresh ref and drops the oldest items past the
	// cap. Only the owner's own adds use it.
	EvictOldest
)

// Pile is one owner's named stack of references.
type Pile struct {
	ID      string
	Owner   string
	Name    string
	Icon    string
	Created time.Time
	Updated time.Time
	// Items are newest first. Hidden or since-deleted entities are still
	// here; filtering is the reader's job, never the store's.
	Items []Item
}

// Refs returns the refs of p's items, newest first.
func (p Pile) Refs() []entity.Ref {
	out := make([]entity.Ref, len(p.Items))
	for i, it := range p.Items {
		out[i] = it.Ref
	}
	return out
}

// Item is one reference on a pile.
type Item struct {
	Ref   entity.Ref
	Added time.Time
}

// Store is the backend contract the [Service] drives. Every per-pile method
// takes the owner and treats a pile of another owner exactly like a missing
// one ([ErrNotFound]); the owner filter lives in the backend's own query or
// lock, never in a separate check.
//
// Implementations must be safe for concurrent use and must apply each
// method atomically: two concurrent adds lose nothing, and limits hold under
// concurrency.
type Store interface {
	// ListPiles returns the owner's piles with their items, oldest pile first.
	ListPiles(ctx context.Context, owner string) ([]Pile, error)
	// GetPile returns one pile with its items.
	GetPile(ctx context.Context, owner, id string) (Pile, error)
	// PileByName finds the owner's pile by case-insensitive name.
	PileByName(ctx context.Context, owner, name string) (Pile, error)
	// CreatePile stores p (ID, Owner, Name, Icon, Created, Updated set by the
	// caller) with refs as its first items, the first ref newest. It returns
	// [ErrLimit] when the owner holds maxPiles, [ErrNameTaken] on a name
	// clash and [ErrIDTaken] when p.ID exists. Duplicate refs collapse; more
	// than maxItems keeps the first maxItems.
	CreatePile(ctx context.Context, p Pile, refs []entity.Ref, maxPiles, maxItems int) error
	// UpdatePile renames or re-icons a pile.
	UpdatePile(ctx context.Context, owner, id, name, icon string, now time.Time) error
	// DeletePile removes a pile and its items.
	DeletePile(ctx context.Context, owner, id string) error
	// AddItems puts refs on top of the pile, the first ref newest. A ref
	// already on the pile keeps its place and does not count as added. What
	// happens past maxItems is overflow's choice, decided in the same atomic
	// step as the add.
	AddItems(
		ctx context.Context, owner, id string, refs []entity.Ref, now time.Time, maxItems int, overflow Overflow,
	) (added int, err error)
	// RemoveItems drops refs from the pile. Refs not on it are ignored.
	RemoveItems(ctx context.Context, owner, id string, refs []entity.Ref) error

	// RenameEntity rewrites every item naming oldID (all faces). Where the
	// new ref is already on a pile the two collapse into one item. Owners are
	// untouched; see RenameOwner.
	RenameEntity(ctx context.Context, oldID, newID string) error
	// DeleteEntity drops every item naming id (all faces). Owners are
	// untouched; see DeleteOwner.
	DeleteEntity(ctx context.Context, id string) error
	// RenameOwner moves every pile owned by oldOwner to newOwner.
	RenameOwner(ctx context.Context, oldOwner, newOwner string) error
	// DeleteOwner drops every pile owned by owner.
	DeleteOwner(ctx context.Context, owner string) error
	// DeleteFace drops every item naming exactly that face of id.
	DeleteFace(ctx context.Context, id string, face entity.Face) error
}
