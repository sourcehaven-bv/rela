package lease

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

// leaseIDRE is the shape of a lease id: 128 random bits in lowercase hex.
var leaseIDRE = regexp.MustCompile(`^[0-9a-f]{32}$`)

// Family is what [Gate.Family] answers about an entity id: its type and the
// faces the caller may read.
type Family struct {
	Type  string
	Faces []entity.Face
}

// Gate is the read gate the service checks access through. Both methods
// resolve the principal on ctx per call, so one Service serves every caller.
//
// A miss (no such entity, or one the caller may not read) is the same error,
// wrapping store.ErrNotFound, for WriteTarget, and ok=false for Family.
type Gate interface {
	// WriteTarget resolves addr (`ID` or `ID@face`) to the one face a write
	// edits. A bare id of a faced type is an ambiguous-address error.
	WriteTarget(ctx context.Context, addr string) (entity.Ref, error)
	// Family returns the readable faces of id.
	Family(ctx context.Context, id string) (Family, bool, error)
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
// Errors: [ErrInvalid] for bad input (before any read); the gate's not-found
// or ambiguous-address error; *acl.ForbiddenError without update access;
// *[HeldError] when the entity has a live lease; [ErrLimit] or [ErrFull]
// past a cap.
func (s *Service) Claim(ctx context.Context, req ClaimRequest) (Claimed, error) {
	ttl, err := checkClaim(req)
	if err != nil {
		return Claimed{}, err
	}
	ref, err := s.gate.WriteTarget(ctx, req.Addr)
	if err != nil {
		return Claimed{}, err
	}
	fam, ok, err := s.gate.Family(ctx, ref.ID)
	if err != nil {
		return Claimed{}, err
	}
	if !ok {
		return Claimed{}, store.ErrNotFound
	}
	if d := s.authorize(ctx, fam.Type, ref); !d.Allow {
		return Claimed{}, &acl.ForbiddenError{Decision: d}
	}
	id, err := newLeaseID()
	if err != nil {
		return Claimed{}, err
	}
	p := principal.From(ctx)
	h := Holder{Identity: Identity(p), User: p.User, Tool: p.Tool, Label: req.Label}
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
	s.hub.wake()
	return Claimed{LeaseID: id, Lease: public(l)}, nil
}

// Renew extends the caller's lease with leaseID by ttl (zero means
// [DefaultTTL]) and replaces its status when status is non-nil.
//
// It returns [ErrLost] when the lease is gone, belongs to someone else, or
// the caller lost read or update access to the entity; in the last case the
// lease is released first.
func (s *Service) Renew(ctx context.Context, leaseID string, ttl time.Duration, status *string) (Lease, error) {
	ttl, err := checkRenew(leaseID, ttl, status)
	if err != nil {
		return Lease{}, err
	}
	p := principal.From(ctx)
	identity, hash := Identity(p), hashLeaseID(leaseID)
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
	// A plain renew changes nothing anyone is shown; a new status does.
	if status != nil && *status != cur.Status {
		s.hub.wake()
	}
	return public(l), nil
}

// Release removes the caller's lease with leaseID. It returns nil whether or
// not the caller held such a lease, so the answer says nothing about other
// people's leases.
func (s *Service) Release(ctx context.Context, leaseID string) error {
	if err := checkLeaseID(leaseID); err != nil {
		return err
	}
	identity, hash := Identity(principal.From(ctx)), hashLeaseID(leaseID)
	now := s.clock.Now()
	_, lookupErr := s.store.Lookup(ctx, hash, identity, now)
	if err := s.store.Release(ctx, hash, identity, now); err != nil {
		return err
	}
	if lookupErr == nil {
		s.hub.wake()
	}
	return nil
}

// Get returns the live leases on ids, keyed by entity id, without hashes or
// identities.
//
// It applies no access check: the caller passes only ids the requester may
// read (a gated list page, a resolved entity).
func (s *Service) Get(ctx context.Context, ids []string) (map[string]Lease, error) {
	got, err := s.store.Get(ctx, ids, s.clock.Now())
	if err != nil {
		return nil, err
	}
	for id, l := range got {
		got[id] = public(l)
	}
	return got, nil
}

// Mine returns the live leases of the principal on ctx, oldest first.
//
// Every one of them passed the caller's access checks at claim or renew; a
// revoked entity is caught at the next renew.
func (s *Service) Mine(ctx context.Context) ([]Lease, error) {
	got, err := s.store.ListByHolder(ctx, Identity(principal.From(ctx)), s.clock.Now())
	if err != nil {
		return nil, err
	}
	for i := range got {
		got[i] = public(got[i])
	}
	return got, nil
}

// EntityRenamed moves the lease from oldID to newID.
func (s *Service) EntityRenamed(ctx context.Context, oldID, newID string) error {
	defer s.hub.wake()
	return s.store.Rekey(ctx, oldID, newID, s.clock.Now())
}

// EntityDeleted drops the lease on the deleted entity, so a recreated entity
// with the same id starts unheld.
func (s *Service) EntityDeleted(ctx context.Context, entityID string) error {
	defer s.hub.wake()
	return s.store.Drop(ctx, entityID, s.clock.Now())
}

// EntityFaceDeleted keeps the lease: it covers the whole family, which still
// exists. Subscribers are woken because the deleted face no longer shows it.
func (s *Service) EntityFaceDeleted(context.Context, string, entity.Face) error {
	s.hub.wake()
	return nil
}

// Identity returns the holder identity of p: the hex SHA-256 of the fields
// [principal.Principal.SameIdentity] compares. A token refresh keeps it.
func Identity(p principal.Principal) string {
	b, err := json.Marshal([]string{p.User, p.Tool, p.RawUser, p.OrgID()})
	if err != nil { // coverage-ignore: unreachable: a string slice always marshals
		panic(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// authorize asks whether the caller may update the face ref of type typ.
func (s *Service) authorize(ctx context.Context, typ string, ref entity.Ref) acl.Decision {
	return s.auth.AuthorizeWrite(ctx, acl.WriteRequest{
		Op: acl.OpUpdate, Subject: acl.NewEntitySubject(typ, ref.ID, ref.Face),
	})
}

// mayUpdate reports whether the caller may still read id and update one of
// its readable faces. A gate error is returned, not read as a denial, so a
// transient failure does not release a lease.
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
