package realtime

import (
	"context"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

// Access answers the access questions of one operation. The wiring site
// builds one per operation over one acl.Request, so every check in the
// operation sees the same ACL snapshot.
type Access interface {
	// Principal is the caller the request is bound to.
	Principal() principal.Principal
	// QueryIdentity is the caller's identity as graph data names it, resolved
	// once by the wiring site. An entity-property role compares the anchor's
	// property against it, with the is_current_user rule. An invalid identity
	// matches nothing.
	QueryIdentity() predicatefns.QueryIdentity
	// HoldsPermissionFor reports whether the caller holds perm for the anchor
	// entity, counting roles conferred by relations to it. The client
	// ceiling's permission axis applies.
	HoldsPermissionFor(ctx context.Context, anchorID, perm string) bool
	// CeilingAdmitsUpdate reports whether the client ceiling alone, not the
	// row ACL, admits an update of entityType.
	CeilingAdmitsUpdate(entityType string) bool
}

// AccessSource opens the [Access] of one operation. The returned context
// carries the same per-request ACL scope, so the anchor and ref reads made
// with it evaluate against the snapshot Access answers from.
//
// A caller the ACL cannot evaluate (an unstamped principal under a
// declarative policy) is an error; the service refuses the operation.
type AccessSource interface {
	For(ctx context.Context) (context.Context, Access, error)
}

// AnchorReader reads the entity a channel hangs off through the gated,
// redacted read path, by the entity GET rule: `ID@face` serves that face
// when the caller may read it, and a bare id serves the face the world
// resolves among the readable ones.
//
// Every miss, including an entity the caller may not read, is
// [ErrAnchorNotFound].
type AnchorReader interface {
	// Anchor reads addr and returns its view. props names the properties the
	// channel type's roles and lifecycle conditions read; the view carries
	// no other property.
	Anchor(ctx context.Context, addr entity.Address, props []string) (AnchorView, error)
}

// AnchorView is what the service learns about an anchor entity.
type AnchorView struct {
	// ID is the entity id.
	ID string
	// Face is the face the caller was admitted through.
	Face entity.Face
	// Type is the stored entity type.
	Type string
	// Props holds the requested properties of the served face, redacted for
	// the caller. The service compares them, and renders only named-role
	// holders (see [SessionWaitingView]).
	Props map[string]any
	// Record is the served face as the `entity` record a lifecycle
	// `condition:` reads, bound from the same redacted properties as Props:
	// a property that was not requested, or that the caller may not see,
	// reads as absent. It is bound whether or not properties were requested.
	// Nil only for a type the schema does not define; a condition then does
	// not hold.
	Record predicate.Value
}

// RefResolver resolves the entity ids that ref properties name, gated and
// redacted for the caller, in one batch per call.
type RefResolver interface {
	// Resolve returns a view for each id the caller may read. An id absent
	// from the result is unreadable or does not exist; the two are not told
	// apart.
	Resolve(ctx context.Context, ids []string) (map[string]RefView, error)
}

// RefView is a readable entity a ref property names: its id, type and
// display title computed from the redacted properties.
type RefView struct {
	ID    string
	Type  string
	Title string
}

// ConfigSource serves the realtime configuration. The service loads it once
// per operation, so a reload applies from the next operation on.
type ConfigSource interface {
	Load() Snapshot
}

// Snapshot is the realtime configuration one operation reads.
type Snapshot struct {
	// Config is nil when realtime is off.
	Config *realtimecfg.Compiled
	// Degraded is set when the configuration is the last valid one, kept
	// after a schema or acl.yaml reload made the file invalid. An
	// entity-property role then grants nobody, and a lifecycle operation
	// with a `condition:` is refused.
	Degraded bool
}

// Clock tells the service the time.
type Clock func() time.Time
