package piles

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Icons is the allowlist of pile icons. The names are rela-components icon
// names; anything else is refused rather than passed to the SPA.
var Icons = []string{"layers", "star", "flag", "bookmark", "inbox", "folder", "tag", "heart", "clock", "target", "bell"}

// idPattern is the shape of a minted pile id. A request naming anything else
// is answered as not found without touching the store.
var idPattern = regexp.MustCompile(`^PIL-[A-Z0-9]{4,16}$`)

const (
	idAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	idLength   = 8
	// idAttempts bounds the retries on an id clash ([ErrIDTaken]). With 32^8
	// ids a second clash in a row means something other than chance.
	idAttempts = 3
)

// ValidID reports whether id has the shape of a pile id.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// OwnerExistsFunc reports whether id names a person entity the ACTING
// principal on ctx can read: the only kind of owner a push may target
// besides the acting user. It must answer through the caller's read gate, so
// that "unknown owner" never reveals a hidden person. A nil func means no
// person mapping is configured, so only the acting user is valid.
type OwnerExistsFunc func(ctx context.Context, id string) (bool, error)

// Clock returns the current time.
type Clock func() time.Time

// Service is the piles API every surface (HTTP, Lua, automation, MCP) uses.
// It resolves the owner from ctx, validates input and enforces limits; the
// [Store] does the atomic work. It never reads the graph: callers resolve
// references through their own visibility gate before handing them over.
type Service struct {
	store       Store
	now         Clock
	ownerExists OwnerExistsFunc
	personType  string
	// declared are the schema's piles by key; declOrder keeps their order.
	declared  map[string]Declared
	declOrder []string
}

// Options are the optional collaborators of [NewService].
type Options struct {
	// Clock defaults to time.Now.
	Clock Clock
	// OwnerExists enables pushes to other users. Nil: only the acting user.
	OwnerExists OwnerExistsFunc
	// PersonType is the ACL's user_entity_type when person mapping is
	// configured, else empty. With it set, every owner is a person entity id
	// (see [Service.Owner]), so renaming or deleting an entity may move or
	// drop piles by owner. Without it owners are login names, which must
	// never be matched against entity ids.
	PersonType string
	// Declared are the piles the schema declares (see [Declared]), in
	// schema order.
	Declared []Declared
}

// NewService returns a Service over st. Nil: st is rejected.
func NewService(st Store, opts Options) (*Service, error) {
	if st == nil {
		return nil, errors.New("piles: NewService requires a Store")
	}
	clock := opts.Clock
	if clock == nil {
		clock = time.Now
	}
	svc := &Service{
		store: st, now: clock, ownerExists: opts.OwnerExists, personType: opts.PersonType,
		declared: make(map[string]Declared, len(opts.Declared)),
	}
	for _, d := range opts.Declared {
		if err := d.validate(); err != nil {
			return nil, err
		}
		if _, dup := svc.declared[d.Key]; dup {
			return nil, fmt.Errorf("%w: declared pile %q appears twice", ErrInvalid, d.Key)
		}
		svc.declared[d.Key] = d
		svc.declOrder = append(svc.declOrder, d.Key)
	}
	return svc, nil
}

// OwnerFrom returns the owner key for the principal on ctx: its User, which
// is the person entity id when person mapping is configured and the login
// name otherwise. No principal, the unknown principal and reserved system
// identities have no piles ([ErrNoOwner]).
func OwnerFrom(ctx context.Context) (string, error) {
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

// Owner returns the acting user's owner key under this service's rules:
// [OwnerFrom], and, when person mapping is configured, only a principal the
// mapping resolved to a person (RawUser set). An unmapped login gets no piles
// then, which keeps login names and entity ids from ever sharing the owner
// key space.
func (s *Service) Owner(ctx context.Context) (string, error) {
	owner, err := OwnerFrom(ctx)
	if err != nil {
		return "", err
	}
	if s.personType != "" && strings.TrimSpace(principal.From(ctx).RawUser) == "" {
		return "", ErrNoOwner
	}
	return owner, nil
}

// List returns the acting user's piles: the declared ones in schema order,
// then the others, oldest first.
//
// Every declared pile is in the answer: one the user does not hold yet is
// created first. That is the only write a list makes, once per user and
// declared pile, so a user sees the schema's piles without having made them.
func (s *Service) List(ctx context.Context) ([]Pile, error) {
	owner, err := s.Owner(ctx)
	if err != nil {
		return nil, err
	}
	ps, err := s.store.ListPiles(ctx, owner)
	if err != nil {
		return nil, err
	}
	if s.ensureDeclared(ctx, owner, ps) {
		if ps, err = s.store.ListPiles(ctx, owner); err != nil {
			return nil, err
		}
	}
	now := s.now().UTC()
	for i := range ps {
		ps[i] = s.decorate(ps[i], now)
	}
	// Declared piles come first, in schema order; the user's follow, oldest
	// first.
	rank := func(p Pile) int {
		if i := slices.Index(s.declOrder, p.Key); i >= 0 && p.Key != "" {
			return i
		}
		return len(s.declOrder)
	}
	slices.SortStableFunc(ps, func(a, b Pile) int { return rank(a) - rank(b) })
	return ps, nil
}

// ensureDeclared creates the declared piles owner does not hold and reports
// whether it created any. A failed create is logged, not returned: the list
// still answers with the piles that exist.
func (s *Service) ensureDeclared(ctx context.Context, owner string, ps []Pile) bool {
	held := make(map[string]bool, len(ps))
	for _, p := range ps {
		held[p.Key] = true
	}
	created := false
	for _, key := range s.declOrder {
		if held[key] {
			continue
		}
		if _, err := s.declaredPile(ctx, owner, s.declared[key]); err != nil {
			slog.WarnContext(ctx, "piles: creating a declared pile failed", "key", key, "err", err)
			continue
		}
		created = true
	}
	return created
}

// Get returns one of the acting user's piles.
func (s *Service) Get(ctx context.Context, id string) (Pile, error) {
	owner, err := s.Owner(ctx)
	if err != nil {
		return Pile{}, err
	}
	if !ValidID(id) {
		return Pile{}, ErrNotFound
	}
	p, err := s.store.GetPile(ctx, owner, id)
	if err != nil {
		return Pile{}, err
	}
	return s.decorate(p, s.now().UTC()), nil
}

// ByKey returns the acting user's declared pile with that key, creating it
// on first use. A key the schema does not declare is [ErrNotFound].
func (s *Service) ByKey(ctx context.Context, key string) (Pile, error) {
	owner, err := s.Owner(ctx)
	if err != nil {
		return Pile{}, err
	}
	d, ok := s.declared[key]
	if !ok {
		return Pile{}, ErrNotFound
	}
	p, err := s.declaredPile(ctx, owner, d)
	if err != nil {
		return Pile{}, err
	}
	return s.decorate(p, s.now().UTC()), nil
}

// ByName returns the acting user's pile with that name (case-insensitive).
// A name that is exactly a declared key returns that declared pile, as in
// [Service.Push].
func (s *Service) ByName(ctx context.Context, name string) (Pile, error) {
	if _, ok := s.declared[name]; ok {
		return s.ByKey(ctx, name)
	}
	owner, err := s.Owner(ctx)
	if err != nil {
		return Pile{}, err
	}
	name, err = cleanName(name)
	if err != nil {
		return Pile{}, err
	}
	return s.store.PileByName(ctx, owner, name)
}

// CreateRequest is the caller-supplied part of a new pile.
type CreateRequest struct {
	Name string
	// Icon defaults to [DefaultIcon].
	Icon string
	// Refs become the first items, the first ref newest.
	Refs []entity.Ref
}

// Create makes a pile for the acting user.
func (s *Service) Create(ctx context.Context, req CreateRequest) (Pile, error) {
	owner, err := s.Owner(ctx)
	if err != nil {
		return Pile{}, err
	}
	return s.create(ctx, owner, req, MaxPiles)
}

// create makes a pile for owner while the owner holds fewer than maxPiles.
func (s *Service) create(ctx context.Context, owner string, req CreateRequest, maxPiles int) (Pile, error) {
	name, err := cleanName(req.Name)
	if err != nil {
		return Pile{}, err
	}
	icon, err := cleanIcon(req.Icon)
	if err != nil {
		return Pile{}, err
	}
	refs, err := cleanRefs(req.Refs)
	if err != nil {
		return Pile{}, err
	}
	now := s.now().UTC()
	p := Pile{Owner: owner, Name: name, Icon: icon, Created: now, Updated: now}
	for range idAttempts {
		if p.ID, err = mintID(); err != nil {
			return Pile{}, err
		}
		err = s.store.CreatePile(ctx, p, refs, maxPiles, MaxItems)
		if !errors.Is(err, ErrIDTaken) {
			break
		}
	}
	if err != nil {
		return Pile{}, err
	}
	return s.store.GetPile(ctx, owner, p.ID)
}

// Update renames and/or re-icons one of the acting user's piles. A nil field
// is left unchanged.
func (s *Service) Update(ctx context.Context, id string, name, icon *string) (Pile, error) {
	owner, err := s.Owner(ctx)
	if err != nil {
		return Pile{}, err
	}
	if !ValidID(id) {
		return Pile{}, ErrNotFound
	}
	cur, err := s.store.GetPile(ctx, owner, id)
	if err != nil {
		return Pile{}, err
	}
	if s.isDeclared(cur) {
		return Pile{}, ErrLocked
	}
	newName, newIcon := cur.Name, cur.Icon
	if name != nil {
		if newName, err = cleanName(*name); err != nil {
			return Pile{}, err
		}
	}
	if icon != nil {
		if newIcon, err = cleanIcon(*icon); err != nil {
			return Pile{}, err
		}
	}
	if err := s.store.UpdatePile(ctx, owner, id, newName, newIcon, s.now().UTC()); err != nil {
		return Pile{}, err
	}
	return s.Get(ctx, id)
}

// Delete removes one of the acting user's piles.
func (s *Service) Delete(ctx context.Context, id string) error {
	owner, err := s.Owner(ctx)
	if err != nil {
		return err
	}
	if !ValidID(id) {
		return ErrNotFound
	}
	cur, err := s.store.GetPile(ctx, owner, id)
	if err != nil {
		return err
	}
	if s.isDeclared(cur) {
		return ErrLocked
	}
	return s.store.DeletePile(ctx, owner, id)
}

// Add puts refs on top of one of the acting user's piles and reports how
// many were new. The oldest items past [MaxItems] are evicted; on a declared
// pile read items go first.
func (s *Service) Add(ctx context.Context, id string, refs []entity.Ref) (int, error) {
	owner, err := s.Owner(ctx)
	if err != nil {
		return 0, err
	}
	if !ValidID(id) {
		return 0, ErrNotFound
	}
	refs, err = cleanRefs(refs)
	if err != nil {
		return 0, err
	}
	p, err := s.store.GetPile(ctx, owner, id)
	if err != nil {
		return 0, err
	}
	now := s.now().UTC()
	opts := AddOptions{Now: now, MaxItems: MaxItems, Overflow: EvictOldest}
	if d, ok := s.declaration(p); ok {
		if err := s.prune(ctx, p, d, now); err != nil {
			return 0, err
		}
		opts.Overflow, opts.Bump, opts.Actor = EvictReadFirst, d.Bump, actorFrom(ctx)
	}
	return s.store.AddItems(ctx, owner, id, refs, opts)
}

// Remove drops refs from one of the acting user's piles. It reports nothing
// about which refs were on the pile, so it cannot be used to probe for them.
func (s *Service) Remove(ctx context.Context, id string, refs []entity.Ref) error {
	owner, err := s.Owner(ctx)
	if err != nil {
		return err
	}
	if !ValidID(id) {
		return ErrNotFound
	}
	refs, err = cleanRefs(refs)
	if err != nil {
		return err
	}
	return s.store.RemoveItems(ctx, owner, id, refs)
}

// PushRequest adds refs to a pile named by Name or Key, possibly another
// user's.
type PushRequest struct {
	// Owner is the target user. Empty means the acting user.
	Owner string
	// Pile is the pile name (case-insensitive). A name that is exactly a
	// declared key addresses that declared pile, so an `add_to_pile` written
	// before the pile was declared keeps reaching it.
	Pile string
	// Key addresses a declared pile instead of Pile. A declared pile is
	// created on first use whatever Create says, counts toward no limit, and
	// evicts read items first when full. Only a push by Key reaches another
	// user's declared pile, and only one with read state.
	Key string
	// Reason is a key of the declared pile's reasons, recorded on the item.
	// Only a push by Key may carry one.
	Reason string
	// SkipSelf makes a push whose target is the acting user a no-op. A
	// notification is about what others do.
	SkipSelf bool
	// Refs are already resolved by the caller through the ACTING principal's
	// read gate; the target owner's gate applies again whenever they read.
	Refs []entity.Ref
	// Create makes the pile, with [DefaultIcon], when it does not exist.
	Create bool
}

// Push adds refs to a pile by name: the acting user's own, or another
// user's when Owner names an existing person (see [OwnerExistsFunc]).
//
// Toward another user a push is write-only and reveals nothing about their
// piles: it always reports 0 added, and every refusal below is silent. It is
// also bounded so it cannot destroy or crowd out what the owner keeps:
//
//   - it never evicts: on a full pile it adds only as many refs as there is
//     room for, the first refs first, and drops the rest ([KeepExisting]);
//   - it creates a missing pile (with Create) only while the owner holds
//     fewer than [MaxForeignPiles] piles; past that the push is dropped;
//   - a missing pile without Create is a no-op.
//
// A declared pile takes a foreign push only by Key and only when it has read
// state (a notification pile); any other foreign push to it is dropped. There
// it evicts read items first, as on the owner's own push.
//
// Each foreign push is logged at Info with the pusher, the target owner and
// the number of refs, never the pile name, which is user data. Only a push to
// the acting user's own pile evicts, reports the real count and errors.
//
// The acting principal must have an owner identity, except for reserved
// system identities (scheduled scripts, system automations), which may push
// to a named owner.
func (s *Service) Push(ctx context.Context, req PushRequest) (int, error) {
	key := req.Key
	if key == "" {
		if _, ok := s.declared[strings.TrimSpace(req.Pile)]; ok {
			key = strings.TrimSpace(req.Pile)
		}
	}
	var decl Declared
	if key != "" {
		var ok bool
		if decl, ok = s.declared[key]; !ok {
			return 0, fmt.Errorf("%w: no declared pile has that key", ErrInvalid)
		}
		if req.Reason != "" {
			if _, ok := decl.Reasons[req.Reason]; !ok {
				return 0, fmt.Errorf("%w: the declared pile has no such reason", ErrInvalid)
			}
		}
	} else if req.Reason != "" {
		return 0, fmt.Errorf("%w: only a declared pile records a reason", ErrInvalid)
	}
	owner, foreign, err := s.pushOwner(ctx, strings.TrimSpace(req.Owner))
	if err != nil {
		return 0, err
	}
	if req.SkipSelf && !foreign {
		return 0, nil
	}
	if foreign && key != "" && (req.Key == "" || !decl.ReadState) {
		// Only a notification reaches another user's declared pile: by Key,
		// onto a pile with read state. A push by name could otherwise fill
		// someone's favorites and evict what they keep.
		return 0, nil
	}
	refs, err := cleanRefs(req.Refs)
	if err != nil {
		return 0, err
	}
	var push func() (int, error)
	if key != "" {
		push = func() (int, error) { return s.pushDeclared(ctx, owner, decl, refs, req.Reason) }
	} else {
		name, nameErr := cleanName(req.Pile)
		if nameErr != nil {
			return 0, nameErr
		}
		lim := ownPush
		if foreign {
			lim = foreignPush
		}
		push = func() (int, error) { return s.pushTo(ctx, owner, name, refs, req.Create, lim) }
	}
	if !foreign {
		return push()
	}
	slog.InfoContext(ctx, "piles: push to another user",
		"pusher", principal.From(ctx).User, "owner", owner, "refs", len(refs))
	_, err = push()
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrLimit) {
		return 0, nil
	}
	return 0, err
}

// pushDeclared adds refs to owner's declared pile d, creating it first when
// owner does not hold it yet.
func (s *Service) pushDeclared(
	ctx context.Context, owner string, d Declared, refs []entity.Ref, reason string,
) (int, error) {
	p, err := s.declaredPile(ctx, owner, d)
	if err != nil {
		return 0, err
	}
	now := s.now().UTC()
	if err := s.prune(ctx, p, d, now); err != nil {
		return 0, err
	}
	return s.store.AddItems(ctx, owner, p.ID, refs, AddOptions{
		Now: now, MaxItems: MaxItems, Overflow: EvictReadFirst, Bump: d.Bump,
		Actor: actorFrom(ctx), Reason: reason,
	})
}

// declaredPile returns owner's pile for d, creating it when owner does not
// hold it. Two first uses may race; the loser reads the winner's pile.
func (s *Service) declaredPile(ctx context.Context, owner string, d Declared) (Pile, error) {
	p, err := s.store.PileByKey(ctx, owner, d.Key)
	if !errors.Is(err, ErrNotFound) {
		return p, err
	}
	now := s.now().UTC()
	np := Pile{Owner: owner, Key: d.Key, Name: d.Name, Icon: d.Icon, Created: now, Updated: now}
	for range idAttempts {
		if np.ID, err = mintID(); err != nil {
			return Pile{}, err
		}
		err = s.store.CreatePile(ctx, np, nil, MaxPiles, MaxItems)
		if !errors.Is(err, ErrIDTaken) {
			break
		}
	}
	if err != nil && !errors.Is(err, ErrKeyTaken) {
		return Pile{}, err
	}
	return s.store.PileByKey(ctx, owner, d.Key)
}

// MarkRead marks refs read on one of the acting user's declared piles with
// read state, or every item when all is set. Like [Service.Remove] it says
// nothing about which refs were on the pile.
func (s *Service) MarkRead(ctx context.Context, id string, refs []entity.Ref, all bool) error {
	owner, err := s.Owner(ctx)
	if err != nil {
		return err
	}
	if !ValidID(id) {
		return ErrNotFound
	}
	refs, err = cleanRefs(refs)
	if err != nil {
		return err
	}
	p, err := s.store.GetPile(ctx, owner, id)
	if err != nil {
		return err
	}
	if d, ok := s.declaration(p); !ok || !d.ReadState {
		return fmt.Errorf("%w: the pile keeps no read state", ErrInvalid)
	}
	return s.store.MarkRead(ctx, owner, id, refs, all, s.now().UTC())
}

// MarkOpened marks refs read on every declared pile of the acting user that
// reads items on open (AutoRead). A pile the user does not hold yet is
// skipped, not created.
func (s *Service) MarkOpened(ctx context.Context, refs []entity.Ref) error {
	owner, err := s.Owner(ctx)
	if err != nil {
		return err
	}
	refs, err = cleanRefs(refs)
	if err != nil || len(refs) == 0 {
		return err
	}
	now := s.now().UTC()
	for _, key := range s.declOrder {
		if !s.declared[key].AutoRead {
			continue
		}
		p, err := s.store.PileByKey(ctx, owner, key)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if err := s.store.MarkRead(ctx, owner, p.ID, refs, false, now); err != nil {
			return err
		}
	}
	return nil
}

// Declaration returns the declaration of the declared pile p, if p is one
// the schema still declares.
func (s *Service) Declaration(p Pile) (Declared, bool) { return s.declaration(p) }

// ActorsArePeople reports that an item's Actor is a person entity id, as
// it is under person mapping. Otherwise it is a login name.
func (s *Service) ActorsArePeople() bool { return s.personType != "" }

func (s *Service) declaration(p Pile) (Declared, bool) {
	if p.Key == "" {
		return Declared{}, false
	}
	d, ok := s.declared[p.Key]
	return d, ok
}

// isDeclared reports whether p is a pile the schema declares. A pile whose
// key the schema no longer declares is an ordinary pile.
func (s *Service) isDeclared(p Pile) bool {
	_, ok := s.declaration(p)
	return ok
}

// decorate gives a declared pile its schema name and icon and hides the
// items its expiry has passed. The store keeps those until the next write
// prunes them, so a read never writes.
func (s *Service) decorate(p Pile, now time.Time) Pile {
	d, ok := s.declaration(p)
	if !ok {
		return p
	}
	p.Name, p.Icon = d.Name, d.Icon
	readBefore, addedBefore := d.cutoffs(now)
	if readBefore.IsZero() && addedBefore.IsZero() {
		return p
	}
	kept := make([]Item, 0, len(p.Items))
	for _, it := range p.Items {
		if expired(it, readBefore, addedBefore) {
			continue
		}
		kept = append(kept, it)
	}
	p.Items = kept
	return p
}

// prune drops the items of declared pile p that d's expiry has passed.
func (s *Service) prune(ctx context.Context, p Pile, d Declared, now time.Time) error {
	readBefore, addedBefore := d.cutoffs(now)
	if readBefore.IsZero() && addedBefore.IsZero() {
		return nil
	}
	return s.store.PruneItems(ctx, p.Owner, p.ID, readBefore, addedBefore)
}

func expired(it Item, readBefore, addedBefore time.Time) bool {
	return (!readBefore.IsZero() && it.Read() && it.ReadAt.Before(readBefore)) ||
		(!addedBefore.IsZero() && it.Added.Before(addedBefore))
}

// actorFrom is the actor an add records: the acting user's owner key, or
// nothing for a system identity, which names no person.
func actorFrom(ctx context.Context) string {
	actor, err := OwnerFrom(ctx)
	if err != nil {
		return ""
	}
	return actor
}

// pushLimits are the bounds a push runs under: the pile cap a create may
// fill and what an add does past the item cap.
type pushLimits struct {
	maxPiles int
	overflow Overflow
}

var (
	ownPush     = pushLimits{maxPiles: MaxPiles, overflow: EvictOldest}
	foreignPush = pushLimits{maxPiles: MaxForeignPiles, overflow: KeepExisting}
)

func (s *Service) pushTo(
	ctx context.Context, owner, name string, refs []entity.Ref, create bool, lim pushLimits,
) (int, error) {
	p, err := s.store.PileByName(ctx, owner, name)
	switch {
	case errors.Is(err, ErrNotFound) && create:
		created, cerr := s.create(ctx, owner, CreateRequest{Name: name, Refs: refs}, lim.maxPiles)
		if errors.Is(cerr, ErrNameTaken) {
			// A concurrent push created it first; add to that one.
			return s.pushTo(ctx, owner, name, refs, false, lim)
		}
		if cerr != nil {
			return 0, cerr
		}
		return len(created.Items), nil
	case err != nil:
		return 0, err
	}
	return s.store.AddItems(ctx, owner, p.ID, refs, AddOptions{
		Now: s.now().UTC(), MaxItems: MaxItems, Overflow: lim.overflow,
	})
}

// pushOwner resolves the push target and reports whether it is another
// user than the acting one.
func (s *Service) pushOwner(ctx context.Context, target string) (owner string, foreign bool, err error) {
	self, selfErr := s.Owner(ctx)
	if target == "" {
		return self, false, selfErr
	}
	if selfErr == nil && target == self {
		return self, false, nil
	}
	// Only a real user or a reserved system identity may push to someone
	// else. No principal, or the unknown one, is refused: anonymity must not
	// be a way to write into a stranger's pile.
	if selfErr != nil {
		p, ok := principal.Stamped(ctx)
		if !ok || !principal.IsReserved(strings.TrimSpace(p.User)) {
			return "", false, ErrNoOwner
		}
	}
	// Owner errors name the rule, never the target: the target may be a
	// login or an email, and callers such as the automation cascade log the
	// error.
	if principal.IsReserved(target) || target == principal.Unknown {
		return "", false, ErrUnknownOwner
	}
	if s.ownerExists == nil {
		return "", false, fmt.Errorf("%w (no person mapping is configured)", ErrUnknownOwner)
	}
	ok, err := s.ownerExists(ctx, target)
	if err != nil {
		return "", false, fmt.Errorf("piles: checking owner: %w", err)
	}
	if !ok {
		return "", false, ErrUnknownOwner
	}
	return target, true, nil
}

// EntityRenamed keeps piles pointing at a renamed entity and, under person
// mapping, moves the piles of a renamed person. It implements
// entitymanager.AliasRewriter, runs for every owner, and is not reachable
// from any request surface. It runs outside any store transaction, as the
// comments hook does, so it cannot roll back with one.
func (s *Service) EntityRenamed(ctx context.Context, oldID, newID string) error {
	if err := s.store.RenameEntity(ctx, oldID, newID); err != nil {
		return err
	}
	if s.personType == "" {
		return nil
	}
	return s.store.RenameOwner(ctx, oldID, newID)
}

// EntityDeleted drops a deleted entity from every pile and, under person
// mapping, drops the piles of a deleted person: ids can be reused, and a new
// person must not inherit a stranger's piles.
func (s *Service) EntityDeleted(ctx context.Context, id string) error {
	if err := s.store.DeleteEntity(ctx, id); err != nil {
		return err
	}
	if s.personType == "" {
		return nil
	}
	return s.store.DeleteOwner(ctx, id)
}

// EntityFaceDeleted drops exactly that face from every pile.
func (s *Service) EntityFaceDeleted(ctx context.Context, id string, face entity.Face) error {
	return s.store.DeleteFace(ctx, id, face)
}

func cleanName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if !utf8.ValidString(name) {
		return "", fmt.Errorf("%w: name is not valid UTF-8", ErrInvalid)
	}
	if utf8.RuneCountInString(name) > MaxNameRunes {
		return "", fmt.Errorf("%w: name is longer than %d characters", ErrInvalid, MaxNameRunes)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%w: name contains a control character", ErrInvalid)
		}
	}
	return name, nil
}

func cleanIcon(icon string) (string, error) {
	if icon == "" {
		return DefaultIcon, nil
	}
	if slices.Contains(Icons, icon) {
		return icon, nil
	}
	// The caller's string is not echoed: naming the rule is enough.
	return "", fmt.Errorf("%w: unknown icon; see the icons list", ErrInvalid)
}

// cleanRefs refuses refs without an id, collapses duplicates keeping the
// first, and caps the list at [MaxItems].
func cleanRefs(refs []entity.Ref) ([]entity.Ref, error) {
	seen := make(map[entity.Ref]bool, len(refs))
	out := make([]entity.Ref, 0, len(refs))
	for _, r := range refs {
		if r.IsZero() {
			return nil, fmt.Errorf("%w: item without an id", ErrInvalid)
		}
		if seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	if len(out) > MaxItems {
		out = out[:MaxItems]
	}
	return out, nil
}

func mintID() (string, error) {
	buf := make([]byte, idLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("piles: minting id: %w", err)
	}
	for i, b := range buf {
		buf[i] = idAlphabet[int(b)%len(idAlphabet)]
	}
	return "PIL-" + string(buf), nil
}
