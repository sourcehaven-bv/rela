// Package attachment exposes the CLI-shaped facade for managing
// entity file attachments (attach, list). The service depends only
// on the focused primitives it needs (Store, Meta, EntityManager) so
// it can be constructed at any wiring site.
package attachment

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Info describes a single file attachment on an entity.
type Info struct {
	Property    string
	Path        string
	FileName    string
	ContentType string
	Size        int64
}

// Result is the outcome of [Service.Attach] and [Service.WriteAttachment].
type Result struct {
	Path     string
	FileName string

	// Entity is the post-write entity as stored, unredacted. A caller that
	// serves it must apply its own read-side redaction.
	Entity *entity.Entity
}

// Stamper is the write surface the attachment service needs: recording an
// attachment changes exactly one file property of one face.
// entitymanager.Attachments supplies it. It is the one write the file-property
// rule lets change a file value (BUG-CTUW2N): a face's value is what grants
// it the bytes it names, so an ordinary write may not set it.
//
// It patches rather than saves the whole entity: a whole-entity save would
// hold the record across the processor run (a scan may take up to a minute)
// and overwrite any property edited meanwhile.
type Stamper interface {
	StampAttachments(ctx context.Context, ref entity.Ref, prop string, value any) (*entity.UpdateResult, error)
}

// Locker is the keyed lock the service takes around each write to one
// entity's attachments. The wiring site passes entitymanager.Attachments, so
// the service shares one lock instance with the manager's copy engine and
// face delete, which also change references.
type Locker interface {
	// Acquire blocks until key is held or ctx is done, and returns an
	// idempotent release. A wait cut short by ctx returns an error wrapping
	// ctx.Err().
	Acquire(ctx context.Context, key string) (release func(), err error)
}

// WriteAuthorizer decides whether the caller in ctx may still change e's
// attachments. The service asks it under the attachment lock, right before the
// store is touched: callers authorize up front too, but a slow upload or scan
// separates that check from the write, and the entity's state or the caller's
// grants may change in between. It returns nil to allow.
type WriteAuthorizer interface {
	AuthorizeAttachmentWrite(ctx context.Context, e *entity.Entity) error
}

// AllowAllWrites is the named opt-out [WriteAuthorizer] for callers on the
// operator's shell (the CLI), which has no ACL to consult.
type AllowAllWrites struct{}

// AuthorizeAttachmentWrite implements [WriteAuthorizer] and allows every write.
func (AllowAllWrites) AuthorizeAttachmentWrite(context.Context, *entity.Entity) error { return nil }

// Deps is the dependency bundle [New] requires. Store, Meta, EntityManager,
// Locker and Authorizer are mandatory; [New] returns an error if any is nil.
// Processor is optional — when nil the service uses [NoopProcessor].
type Deps struct {
	Store         store.Store
	Meta          *metamodel.Metamodel
	EntityManager Stamper

	// Locker serializes writers to one entity's attachments; see
	// [Service.WriteAttachment].
	Locker Locker

	// Authorizer re-checks each write under the attachment lock. Use
	// [AllowAllWrites] where there is no ACL.
	Authorizer WriteAuthorizer

	// Processor inspects/rewrites attachment bytes before they are persisted
	// (scan, MIME validation, transform). Optional; defaults to [NoopProcessor].
	Processor Processor
}

// Service implements the attachment-facade methods that CLI invokes.
// Constructed once at the wiring site and shared across subcommands.
type Service struct {
	deps Deps
}

// New constructs a Service. Returns an error if any required
// dependency is nil — CLAUDE.md "constructors reject nil required
// fields."
func New(d Deps) (*Service, error) {
	if d.Store == nil {
		return nil, errors.New("attachment: Store is required")
	}
	if d.Meta == nil {
		return nil, errors.New("attachment: Meta is required")
	}
	if d.EntityManager == nil {
		return nil, errors.New("attachment: EntityManager is required")
	}
	if d.Locker == nil {
		return nil, errors.New("attachment: Locker is required")
	}
	if d.Authorizer == nil {
		return nil, errors.New("attachment: Authorizer is required")
	}
	if d.Processor == nil {
		d.Processor = NoopProcessor{}
	}
	return &Service{deps: d}, nil
}
