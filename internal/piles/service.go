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
var Icons = []string{"layers", "star", "flag", "bookmark", "inbox", "folder", "tag", "heart", "clock", "target"}

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
	return &Service{store: st, now: clock, ownerExists: opts.OwnerExists, personType: opts.PersonType}, nil
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

// List returns the acting user's piles, oldest pile first.
func (s *Service) List(ctx context.Context) ([]Pile, error) {
	owner, err := s.Owner(ctx)
	if err != nil {
		return nil, err
	}
	return s.store.ListPiles(ctx, owner)
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
	return s.store.GetPile(ctx, owner, id)
}

// ByName returns the acting user's pile with that name (case-insensitive).
func (s *Service) ByName(ctx context.Context, name string) (Pile, error) {
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
	var newName, newIcon string
	if name == nil || icon == nil {
		// Only a partial update needs the current values; a full one learns
		// about a missing pile from UpdatePile.
		cur, gerr := s.store.GetPile(ctx, owner, id)
		if gerr != nil {
			return Pile{}, gerr
		}
		newName, newIcon = cur.Name, cur.Icon
	}
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
	return s.store.GetPile(ctx, owner, id)
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
	return s.store.DeletePile(ctx, owner, id)
}

// Add puts refs on top of one of the acting user's piles and reports how
// many were new. The oldest items past [MaxItems] are evicted.
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
	return s.store.AddItems(ctx, owner, id, refs, s.now().UTC(), MaxItems, EvictOldest)
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

// PushRequest adds refs to a pile named by Name, possibly another user's.
type PushRequest struct {
	// Owner is the target user. Empty means the acting user.
	Owner string
	// Pile is the pile name (case-insensitive).
	Pile string
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
// Each foreign push is logged at Info with the pusher, the target owner and
// the number of refs, never the pile name, which is user data. Only a push to
// the acting user's own pile evicts, reports the real count and errors.
//
// The acting principal must have an owner identity, except for reserved
// system identities (scheduled scripts, system automations), which may push
// to a named owner.
func (s *Service) Push(ctx context.Context, req PushRequest) (int, error) {
	owner, foreign, err := s.pushOwner(ctx, strings.TrimSpace(req.Owner))
	if err != nil {
		return 0, err
	}
	name, err := cleanName(req.Pile)
	if err != nil {
		return 0, err
	}
	refs, err := cleanRefs(req.Refs)
	if err != nil {
		return 0, err
	}
	if !foreign {
		return s.pushTo(ctx, owner, name, refs, req.Create, ownPush)
	}
	slog.InfoContext(ctx, "piles: push to another user",
		"pusher", principal.From(ctx).User, "owner", owner, "refs", len(refs))
	_, err = s.pushTo(ctx, owner, name, refs, req.Create, foreignPush)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrLimit) {
		return 0, nil
	}
	return 0, err
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
	return s.store.AddItems(ctx, owner, p.ID, refs, s.now().UTC(), MaxItems, lim.overflow)
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
