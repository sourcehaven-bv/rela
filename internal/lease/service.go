package lease

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

const (
	// DefaultTTL is the lease duration when a caller names none.
	DefaultTTL = 15 * time.Minute
	// MinTTL and MaxTTL bound a caller-chosen lease duration.
	MinTTL = 30 * time.Second
	MaxTTL = time.Hour
	// DefaultPollInterval is how often [Service.Watch] polls the store version.
	DefaultPollInterval = time.Second
)

// ErrNoPrincipal reports a claim, renew or release without a user principal
// on the context: none stamped, a zero one, or the [principal.Unknown]
// attribution default.
var ErrNoPrincipal = errors.New("lease: no principal on the request")

// leaseIDRE is the shape of a lease id: 128 random bits in lowercase hex.
var leaseIDRE = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Family is what [Gate.Family] answers about an entity id: its type and the
// faces the caller may read.
type Family struct {
	Type  string
	Faces []entity.Face
}

// Gate is the read gate the service checks access through. Every method
// resolves the principal on ctx per call, so one Service serves every caller.
//
// A miss (no such entity, or one the caller may not read) is
// store.ErrNotFound itself, unwrapped, for WriteTarget, ok=false for Family,
// and an absent key for Readable. [Service.Claim] answers every miss with
// that same value, including an address that does not parse and an entity
// that goes away between its two reads.
type Gate interface {
	// WriteTarget resolves addr (`ID` or `ID@face`) to the one face a write
	// edits. A bare id of a faced type is an *[AmbiguousError].
	WriteTarget(ctx context.Context, addr string) (entity.Ref, error)
	// Family returns the readable faces of id.
	Family(ctx context.Context, id string) (Family, bool, error)
	// Readable returns, as true keys, the ids in ids whose entity the caller
	// may read some face of. It is one batch read: its store round trips do
	// not grow with len(ids).
	Readable(ctx context.Context, ids []string) (map[string]bool, error)
}

// WriteAuthorizer decides whether the principal on ctx may write.
type WriteAuthorizer interface {
	AuthorizeWrite(ctx context.Context, req acl.WriteRequest) acl.Decision
}

// Clock tells the service the time.
type Clock interface {
	Now() time.Time
}

// SystemClock is the wall clock.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }

// Deps are the collaborators of a [Service]. Every field but PollInterval is
// required.
type Deps struct {
	Store      Store
	Gate       Gate
	Authorizer WriteAuthorizer
	Clock      Clock
	// Hub carries the wake-ups. Services over one store share it.
	Hub *Hub
	// PollInterval is how often Watch polls the store version. Zero means
	// [DefaultPollInterval].
	PollInterval time.Duration
}

// Service applies access checks, validation and wake-ups on top of a
// [Store]. It is safe for concurrent use.
//
// # Access
//
// A claim needs read access (a miss is the same not-found error as for an
// entity that does not exist) and then update access on the face
// [Gate.WriteTarget] picks. Only then does the store see the claim, so a
// caller without update access never learns who holds the entity.
//
// A renew runs the checks again against the leased entity id. A lease is
// keyed by the whole face family, so the re-check passes when the caller may
// update any face it can read. When it fails, the lease is released and the
// renew reports [ErrLost], the same as an expired lease.
//
// The two checks ask about different faces and still answer one question.
// A claim names an address, so it asks about the one face
// [Gate.WriteTarget] resolves it to. A renew holds only the entity id, so it
// asks about every readable face. The lease covers the family, so both ask
// "may this caller work on this entity". A caller who claimed through face A
// and can now update only face B could claim again through B, so the renew
// keeps the lease rather than make them do that.
//
// [Service.Mine] gates its answer through [Gate.Readable]. One identity
// covers every session of a user, a browser session and an agent token
// alike, whatever their scopes. So a lease claimed in one session must not
// show an entity to another session whose ceiling hides it.
//
// Claim, Renew and Release refuse a caller without a principal on ctx with
// [ErrNoPrincipal], before any read: every such caller would share one
// empty identity.
//
// # Re-claiming your own lease
//
// A claim on an entity the caller already holds fails with *[HeldError]
// naming the caller, as the store does. Agents that share one identity must
// not take over each other's work silently; the holder renews instead.
//
// # Secrets
//
// The lease id is returned by Claim only. The store sees its SHA-256, and no
// log line carries the lease id, its hash or the status text.
type Service struct {
	store Store
	gate  Gate
	auth  WriteAuthorizer
	clock Clock
	poll  time.Duration
	hub   *Hub
}

// NewService returns a service over deps.
//
// Nil: every collaborator is rejected; a missing one would otherwise fail
// on the first claim instead of at wiring.
func NewService(deps Deps) (*Service, error) {
	switch {
	case deps.Store == nil:
		return nil, errors.New("lease: NewService requires a Store")
	case deps.Gate == nil:
		return nil, errors.New("lease: NewService requires a Gate")
	case deps.Authorizer == nil:
		return nil, errors.New("lease: NewService requires a WriteAuthorizer")
	case deps.Clock == nil:
		return nil, errors.New("lease: NewService requires a Clock")
	case deps.Hub == nil:
		return nil, errors.New("lease: NewService requires a Hub")
	}
	poll := deps.PollInterval
	if poll <= 0 {
		poll = DefaultPollInterval
	}
	return &Service{
		store: deps.Store, gate: deps.Gate, auth: deps.Authorizer, clock: deps.Clock, poll: poll,
		hub: deps.Hub,
	}, nil
}

// ClaimRequest is the input of [Service.Claim].
type ClaimRequest struct {
	// Addr is the entity address: `ID`, or `ID@face` for a faced type.
	Addr string
	// TTL is the lease duration; zero means [DefaultTTL].
	TTL time.Duration
	// Label tells apart agents that share one identity.
	Label string
	// Status is a short note on what the holder is doing.
	Status string
	// Type, when set, is the entity type the caller addressed, as in an
	// HTTP route that names the type's plural. An entity of another type
	// is not found, the same answer as for one that does not exist.
	Type string
}

// Claimed is the result of a successful claim.
type Claimed struct {
	// LeaseID is the secret that renews and releases the lease. It is
	// returned here once and stored nowhere.
	LeaseID string
	// Lease is the new lease, without its hash.
	Lease Lease
}

// Claim takes the lease on the entity at req.Addr for the principal on ctx.
//
// The checks run in a fixed order: the family read first, so that an entity
// the caller may not read, or one of another type than req.Type, is not found
// before the write target is resolved or update access is asked about; then
// the write target; then update access on it.
//
// Errors: [ErrInvalid] for bad input (before any read); store.ErrNotFound or
// *[AmbiguousError] from the gate; *acl.ForbiddenError without update
// access; *[HeldError] when the entity has a live lease; [ErrLimit] or
// [ErrFull] past a cap.
func (s *Service) Claim(ctx context.Context, req ClaimRequest) (Claimed, error) {
	p, err := caller(ctx)
	if err != nil {
		return Claimed{}, err
	}
	ttl, err := checkClaim(req)
	if err != nil {
		return Claimed{}, err
	}
	addr, err := entity.ParseAddress(req.Addr)
	if err != nil {
		return Claimed{}, store.ErrNotFound
	}
	fam, ok, err := s.gate.Family(ctx, addr.ID())
	if err != nil {
		return Claimed{}, err
	}
	if !ok || (req.Type != "" && req.Type != fam.Type) {
		return Claimed{}, store.ErrNotFound
	}
	ref, err := s.gate.WriteTarget(ctx, req.Addr)
	if err != nil {
		return Claimed{}, err
	}
	// fam.Type is from the first read: a concurrent delete and recreate
	// under another type is accepted, which an advisory lease tolerates.
	if d := s.authorize(ctx, fam.Type, ref); !d.Allow {
		return Claimed{}, &acl.ForbiddenError{Decision: d}
	}
	id, err := newLeaseID()
	if err != nil {
		return Claimed{}, err
	}
	h := Holder{Identity: principal.Identity(p), User: p.User, Tool: p.Tool, Label: req.Label}
	now := s.clock.Now()
	l, err := s.store.Claim(ctx, ref.ID, h, hashLeaseID(id), now, now.Add(ttl), req.Status)
	var held *HeldError
	if errors.As(err, &held) {
		slog.Info("lease: claim refused, entity is held", "user", p.User, "tool", p.Tool, "entity", ref.ID)
		return Claimed{}, &HeldError{Current: public(held.Current)}
	}
	if err != nil {
		return Claimed{}, err
	}
	slog.Info("lease: claimed", "user", p.User, "tool", p.Tool, "entity", ref.ID)
	s.hub.wake()
	return Claimed{LeaseID: id, Lease: public(l)}, nil
}

// Renew extends the caller's lease with leaseID by ttl (zero means
// [DefaultTTL]) and replaces its status when status is non-nil.
//
// It returns [ErrLost] when the lease is gone, belongs to someone else, or
// the caller lost read or update access to the entity; in the last case the
// lease is released first.
//
// A rename can move the lease between the lookup and the access check, so
// the check misses on the old id. A miss therefore looks the lease up once
// more and, when its id changed, checks the new id before it releases.
func (s *Service) Renew(ctx context.Context, leaseID string, ttl time.Duration, status *string) (Lease, error) {
	p, err := caller(ctx)
	if err != nil {
		return Lease{}, err
	}
	ttl, err = checkRenew(leaseID, ttl, status)
	if err != nil {
		return Lease{}, err
	}
	identity, hash := principal.Identity(p), hashLeaseID(leaseID)
	now := s.clock.Now()
	cur, err := s.store.Lookup(ctx, hash, identity, now)
	if err != nil {
		return Lease{}, err
	}
	allowed, err := s.mayUpdate(ctx, cur.ID)
	if err != nil {
		return Lease{}, err
	}
	if !allowed {
		again, lookupErr := s.store.Lookup(ctx, hash, identity, now)
		if lookupErr != nil {
			return Lease{}, lookupErr
		}
		if again.ID != cur.ID {
			if allowed, err = s.mayUpdate(ctx, again.ID); err != nil {
				return Lease{}, err
			}
			cur = again
		}
	}
	if !allowed {
		slog.Debug("lease: renew lost access; lease released", "user", p.User, "tool", p.Tool, "entity", cur.ID)
		if relErr := s.store.Release(ctx, hash, identity, now); relErr != nil {
			return Lease{}, relErr
		}
		s.hub.wake()
		return Lease{}, ErrLost
	}
	l, err := s.store.Renew(ctx, hash, identity, now, now.Add(ttl), status)
	if err != nil {
		return Lease{}, err
	}
	// The same rule as the store's version: a plain extension wakes nobody,
	// a new status or an earlier expiry does.
	statusChanged := status != nil && *status != cur.Status
	if statusChanged {
		slog.Info("lease: status changed", "user", p.User, "tool", p.Tool, "entity", l.ID)
	}
	if statusChanged || l.Expires.Before(cur.Expires) {
		s.hub.wake()
	}
	return public(l), nil
}

// Release removes the caller's lease with leaseID. It returns nil whether or
// not the caller held such a lease, so the answer says nothing about other
// people's leases.
//
// It wakes subscribers after every successful release, whether or not a
// lease went: wakes coalesce, and telling the cases apart would cost a read
// on every release.
func (s *Service) Release(ctx context.Context, leaseID string) error {
	p, err := caller(ctx)
	if err != nil {
		return err
	}
	if err := checkLeaseID(leaseID); err != nil {
		return err
	}
	if err := s.store.Release(ctx, hashLeaseID(leaseID), principal.Identity(p), s.clock.Now()); err != nil {
		return err
	}
	s.hub.wake()
	return nil
}

// GetReadable returns the live leases on ids, keyed by entity id, without
// hashes or identities. It is one store read, so a list page costs the same
// at any length.
//
// Contract: every id in ids is one the requester may read. GetReadable
// applies no access check, so the caller passes the ids of a gated list page
// or a resolved entity, never ids taken from the request.
func (s *Service) GetReadable(ctx context.Context, ids []string) (map[string]Lease, error) {
	got, err := s.store.Get(ctx, ids, s.clock.Now())
	if err != nil {
		return nil, err
	}
	for id, l := range got {
		got[id] = public(l)
	}
	return got, nil
}

// Mine returns the live leases of the principal on ctx that the caller may
// read now, oldest first.
//
// Another session of the same identity may hold a lease on an entity this
// caller's ceiling hides; that lease is left out, as if it did not exist.
// The ids (at most [MaxPerIdentity]) are gated in one [Gate.Readable] batch.
func (s *Service) Mine(ctx context.Context) ([]Lease, error) {
	got, err := s.store.ListByHolder(ctx, principal.Identity(principal.From(ctx)), s.clock.Now())
	if err != nil || len(got) == 0 {
		return nil, err
	}
	ids := make([]string, len(got))
	for i, l := range got {
		ids[i] = l.ID
	}
	readable, err := s.gate.Readable(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Lease, 0, len(got))
	for _, l := range got {
		if readable[l.ID] {
			out = append(out, public(l))
		}
	}
	return out, nil
}

// EntityRenamed moves the lease from oldID to newID.
//
// The rename has happened, so the move must not depend on the requester
// staying connected: it runs without the cancellation of ctx.
func (s *Service) EntityRenamed(ctx context.Context, oldID, newID string) error {
	defer s.hub.wake()
	return s.store.Rekey(context.WithoutCancel(ctx), oldID, newID, s.clock.Now())
}

// EntityDeleted drops the lease on the deleted entity, so a recreated entity
// with the same id starts unheld. Like [Service.EntityRenamed] it ignores
// the cancellation of ctx.
func (s *Service) EntityDeleted(ctx context.Context, entityID string) error {
	defer s.hub.wake()
	return s.store.Drop(context.WithoutCancel(ctx), entityID, s.clock.Now())
}

// EntityFaceDeleted keeps the lease: it covers the whole family, which still
// exists. Subscribers are woken because the deleted face no longer shows it.
func (s *Service) EntityFaceDeleted(context.Context, string, entity.Face) error {
	s.hub.wake()
	return nil
}

// authorize asks whether the caller may update the face ref of type typ.
func (s *Service) authorize(ctx context.Context, typ string, ref entity.Ref) acl.Decision {
	return s.auth.AuthorizeWrite(ctx, acl.WriteRequest{
		Op: acl.OpUpdate, Subject: acl.NewEntitySubject(typ, ref.ID, ref.Face),
	})
}

// mayUpdate reports whether the caller may still read id and update one of
// its readable faces.
//
// A gate error is returned, not read as a denial, and keeps the lease. The
// gated reader the wiring supplies answers gate and store failures as a
// miss (ok=false) instead, so with it a transient failure does release the
// lease: the check fails closed.
func (s *Service) mayUpdate(ctx context.Context, id string) (bool, error) {
	fam, ok, err := s.gate.Family(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if !ok {
		return false, nil
	}
	return slices.ContainsFunc(fam.Faces, func(f entity.Face) bool {
		return s.authorize(ctx, fam.Type, entity.Ref{ID: id, Face: f}).Allow
	}), nil
}

// caller returns the principal on ctx, or [ErrNoPrincipal] when none was
// stamped, it is zero, or its user is [principal.Unknown], the attribution
// default that names no user.
func caller(ctx context.Context) (principal.Principal, error) {
	p, ok := principal.Stamped(ctx)
	if !ok || p.IsZero() || p.User == principal.Unknown {
		return principal.Principal{}, ErrNoPrincipal
	}
	return p, nil
}

// public strips what the store keeps for matching: the lease hash and the
// holder identity.
func public(l Lease) Lease {
	l.LeaseHash = ""
	l.Holder.Identity = ""
	return l
}

// checkClaim validates req and returns its lease duration.
func checkClaim(req ClaimRequest) (time.Duration, error) {
	if req.Addr == "" {
		return 0, fmt.Errorf("%w: entity address must be non-empty", ErrInvalid)
	}
	if err := CheckLabel(req.Label); err != nil {
		return 0, err
	}
	if err := CheckStatus(req.Status); err != nil {
		return 0, err
	}
	return checkTTL(req.TTL)
}

// checkRenew validates the arguments of [Service.Renew] and returns the lease
// duration.
func checkRenew(leaseID string, ttl time.Duration, status *string) (time.Duration, error) {
	if err := checkLeaseID(leaseID); err != nil {
		return 0, err
	}
	if status != nil {
		if err := CheckStatus(*status); err != nil {
			return 0, err
		}
	}
	return checkTTL(ttl)
}

// checkTTL returns ttl, or [DefaultTTL] for zero, within [MinTTL, MaxTTL].
func checkTTL(ttl time.Duration) (time.Duration, error) {
	if ttl == 0 {
		return DefaultTTL, nil
	}
	if ttl < MinTTL || ttl > MaxTTL {
		return 0, fmt.Errorf("%w: ttl must be between %s and %s", ErrInvalid, MinTTL, MaxTTL)
	}
	return ttl, nil
}

// checkLeaseID refuses anything that is not the shape [newLeaseID] returns.
func checkLeaseID(id string) error {
	if !leaseIDRE.MatchString(id) {
		return fmt.Errorf("%w: malformed lease id", ErrInvalid)
	}
	return nil
}

// newLeaseID returns 128 bits from crypto/rand in lowercase hex.
func newLeaseID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil { // coverage-ignore: crypto/rand does not fail on supported platforms
		return "", fmt.Errorf("lease: generate lease id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// hashLeaseID returns the lease hash the store keys a lease id by.
func hashLeaseID(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}
