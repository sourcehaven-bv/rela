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
// # Declared piles
//
// The schema may declare piles under its top-level `piles:` key (see
// [Declared]), such as favorites or notifications. Every user holds each
// one, made on first use and found by its key. A declared pile is locked:
// its name and icon come from the schema, it counts toward no limit, and it
// evicts read items first, so a push by another user always lands. Its items
// may record who caused them, why, and when the owner read them; the schema
// can expire read or old items, which the next write prunes and every read
// hides.
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
	// ErrKeyTaken reports a declared pile the owner already holds. Two
	// requests that both create it on first use race to it; the loser reads
	// the winner's pile.
	ErrKeyTaken = errors.New("piles: declared pile already exists")
	// ErrLocked reports a rename, re-icon or delete of a declared pile. Its
	// name and icon come from the schema, and it exists for every user.
	ErrLocked = errors.New("piles: declared pile cannot be changed")
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
	// EvictReadFirst adds every fresh ref and, past the cap, drops read items
	// before unread ones, the oldest first within each. Declared piles use it
	// for every add, a push by another user included: a declared pile is
	// filled by the system, not collected by hand, so a full one must keep
	// taking new notifications.
	EvictReadFirst
)

// AddOptions are the parameters of [Store.AddItems].
type AddOptions struct {
	// Now stamps the added items.
	Now time.Time
	// MaxItems is the item cap.
	MaxItems int
	// Overflow says what happens past MaxItems.
	Overflow Overflow
	// Bump moves a ref already on the pile to the top, restamps it with Now,
	// Actor and Reason, and marks it unread. Without it such a ref keeps its
	// place and does not count as added.
	Bump bool
	// Actor is the user whose write caused the add; empty for none.
	Actor string
	// Reason is a key of the declared pile's reasons; empty for none.
	Reason string
}

// Pile is one owner's named stack of references.
type Pile struct {
	ID    string
	Owner string
	// Key is the schema key of a declared pile, empty for a pile the user
	// made. The service overlays a declared pile's Name and Icon from the
	// schema, so the stored ones are only what they were at creation.
	Key     string
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
	// Actor is the user whose write put the item here; empty when unknown.
	Actor string
	// Reason is the declared pile's reason key; empty for none.
	Reason string
	// ReadAt is when the owner read the item; zero while unread.
	ReadAt time.Time
}

// Read reports whether the owner has read the item.
func (it Item) Read() bool { return !it.ReadAt.IsZero() }

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
	// PileByName finds the owner's pile by case-insensitive name, among the
	// piles without a Key.
	PileByName(ctx context.Context, owner, name string) (Pile, error)
	// PileByKey finds the owner's pile with that Key.
	PileByKey(ctx context.Context, owner, key string) (Pile, error)
	// CreatePile stores p (ID, Owner, Key, Name, Icon, Created, Updated set
	// by the caller) with refs as its first items, the first ref newest.
	//
	// A pile without a Key returns [ErrLimit] when the owner holds maxPiles
	// such piles and [ErrNameTaken] when one of them has the name. A pile
	// with a Key counts toward no limit and clashes with no name; it returns
	// [ErrKeyTaken] when the owner holds that Key. Any pile returns
	// [ErrIDTaken] when p.ID exists. Duplicate refs collapse; more than
	// maxItems keeps the first maxItems.
	CreatePile(ctx context.Context, p Pile, refs []entity.Ref, maxPiles, maxItems int) error
	// UpdatePile renames or re-icons a pile.
	UpdatePile(ctx context.Context, owner, id, name, icon string, now time.Time) error
	// DeletePile removes a pile and its items.
	DeletePile(ctx context.Context, owner, id string) error
	// AddItems puts refs on top of the pile, the first ref newest, and
	// reports how many were added or bumped. A ref already on the pile keeps
	// its place and does not count unless opts.Bump is set. What happens past
	// opts.MaxItems is opts.Overflow's choice, decided in the same atomic
	// step as the add.
	AddItems(ctx context.Context, owner, id string, refs []entity.Ref, opts AddOptions) (added int, err error)
	// RemoveItems drops refs from the pile. Refs not on it are ignored.
	RemoveItems(ctx context.Context, owner, id string, refs []entity.Ref) error
	// MarkRead stamps the unread items naming refs with now, or every unread
	// item when all is set. Read items keep their stamp. Refs not on the
	// pile are ignored.
	MarkRead(ctx context.Context, owner, id string, refs []entity.Ref, all bool, now time.Time) error
	// PruneItems drops items read before readBefore and items added before
	// addedBefore. A zero time disables that half.
	PruneItems(ctx context.Context, owner, id string, readBefore, addedBefore time.Time) error

	// RenameEntity rewrites every item naming oldID (all faces). Where the
	// new ref is already on a pile the two collapse into one item. Owners are
	// untouched; see RenameOwner.
	RenameEntity(ctx context.Context, oldID, newID string) error
	// DeleteEntity drops every item naming id (all faces). Owners are
	// untouched; see DeleteOwner.
	DeleteEntity(ctx context.Context, id string) error
	// RenameOwner moves every pile owned by oldOwner to newOwner, and sets
	// newOwner as the actor of every item oldOwner caused, in any pile.
	RenameOwner(ctx context.Context, oldOwner, newOwner string) error
	// DeleteOwner drops every pile owned by owner, and clears owner as the
	// actor of every item in any pile, so a later entity with the same id is
	// not named as its sender.
	DeleteOwner(ctx context.Context, owner string) error
	// DeleteFace drops every item naming exactly that face of id.
	DeleteFace(ctx context.Context, id string, face entity.Face) error
}
