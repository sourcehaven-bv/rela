package visibility

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Loader is the raw read the [Resolver] needs. Satisfied by store.Store. It
// is [EntityGetter] plus ListEntities; EntityGetter remains for the tracer
// and ScriptReader, which never query a world.
//
// [Resolver.Family] reads content-free headers. It gets them through
// [store.ListEntityHeaders], so a Loader that also implements
// [store.HeaderReader] never loads a body there, and one that does not falls
// back to ListEntities.
type Loader interface {
	GetEntity(ctx context.Context, ref entity.Ref) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
}

// World is the world a single-entity read is made in. Build one with
// [WorldOf] or [DeniedWorld].
//
// The zero value has an unset scope. [Resolver.Ref] and [Resolver.Family]
// accept it, because they read no world (the scope only labels provenance
// there); [Resolver.InWorld] refuses it with [store.ErrInvalidQuery]
// (TKT-7IZHP0 design A4), so a forgotten world cannot pass for the trivial
// one.
type World struct {
	scope  store.WorldScope
	denied bool
}

// WorldOf is the world that scope compiles to.
func WorldOf(scope store.WorldScope) World { return World{scope: scope} }

// trivialWorld is the world a script reader starts in until its wiring calls
// WithWorld: the trivial scope, every entity at its implicit face. TKT-7IZHP0
// PR 5a makes the wiring pass the configured default world and removes this.
func trivialWorld() World { return World{scope: store.TrivialScope()} }

// DeniedWorld is a world that exists but that the principal holds no read
// grant for. Every read in it misses, whatever address it names.
func DeniedWorld() World { return World{denied: true} }

// Resolved is one gated, redacted face row, plus the world rule that
// accounts for its face.
//
// Via and ChainPosition come from [store.WorldScope.RuleAt] for the served
// face, in every mode. For an explicit address that is the rule the world
// would give that face, not a claim that the world chose it; the caller
// decides how to present it.
type Resolved struct {
	// Entity is the redacted row. It is never nil when the read succeeded.
	// When nothing is hidden it is the stored row itself (see [Redact]), so
	// callers must not mutate it.
	Entity        *entity.Entity
	Via           store.ResolutionRule
	ChainPosition int
}

// Family is every face of one entity that the principal may read.
type Family struct {
	ID   string
	Type string
	// Faces holds the readable faces in declaration order when the
	// resolver was built [WithFaceOrder], so the order is the schema's and
	// the same on every backend; without it, in token order. A caller that
	// picks a face by position (Faces[0]) must use a resolver built with the
	// option. It is never empty on a hit.
	Faces []entity.Face
}

// Resolver is the one gated read of a single entity. Every mode applies the
// same gates in the same order:
//
//  1. A denied world, or an empty id, is a miss.
//  2. The row gate on the bare id. A gate error is returned; a deny is a
//     miss.
//  3. The principal's readable faces of the type. None is a miss before any
//     query (RR-Z23T2T).
//  4. The load.
//  5. The stored type must equal the claimed type (RR-SRZK6X).
//  6. The face gate on the loaded face.
//  7. Redaction, once.
//
// A miss is (zero, false, nil) and covers denied, missing, type-mismatched,
// face-denied, world-denied and a failed load: the caller cannot tell them
// apart, so a miss is never an existence oracle. A load failure other than
// [store.ErrNotFound] is logged with slog.Warn before it becomes a miss
// (RR-FE1EGP). Only a gate failure (step 2 or 3) is returned as an error: it
// happens before any load, so it discloses nothing about the row.
//
// The policy and allow-all capabilities are the same type with different
// collaborators; see [NewResolver] and [NewAllowAllResolver].
type Resolver struct {
	gate      RowGate
	redact    FieldRedactor
	load      Loader
	faceOrder FaceOrder
}

// FaceOrder returns an entity type's declared face names in declaration
// order (metamodel.FaceOrderOf). visibility may not import the metamodel, so
// the wiring site supplies it.
type FaceOrder func(entityType string) []string

// ResolverOption configures an optional part of a [Resolver].
type ResolverOption func(*Resolver) error

// WithFaceOrder orders [Family.Faces] by order: the implicit face first, then
// the declared faces in declaration order, then any stored face the order
// does not name, by token. Without it the declared faces are ordered by
// token. The design (TKT-7IZHP0 §3.1) makes declaration order the order every
// face listing uses.
// Nil: rejected — an absent option is how a caller asks for token order.
func WithFaceOrder(order FaceOrder) ResolverOption {
	return func(r *Resolver) error {
		if order == nil {
			return errors.New("visibility: WithFaceOrder: order must be non-nil")
		}
		r.faceOrder = order
		return nil
	}
}

// NewResolver builds a policy-enforcing Resolver. All collaborators are
// required.
func NewResolver(gate RowGate, redact FieldRedactor, load Loader, opts ...ResolverOption) (*Resolver, error) {
	if gate == nil {
		return nil, errors.New("visibility: NewResolver: gate must be non-nil")
	}
	if redact == nil {
		return nil, errors.New("visibility: NewResolver: redact must be non-nil")
	}
	if load == nil {
		return nil, errors.New("visibility: NewResolver: load must be non-nil")
	}
	r := &Resolver{gate: gate, redact: redact, load: load}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// NewAllowAllResolver builds the ungated Resolver: [NopGate] and
// [NopRedactor] over load. It is a capability handed out at a wiring site,
// like [AllowAllReader]. It keeps the stored-type check, which is part of
// the read contract rather than of policy.
//
// opts configure it as they configure [NewResolver].
func NewAllowAllResolver(load Loader, opts ...ResolverOption) (*Resolver, error) {
	if load == nil {
		return nil, errors.New("visibility: NewAllowAllResolver: load must be non-nil")
	}
	r := &Resolver{gate: NopGate{}, redact: NopRedactor{}, load: load}
	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// Address resolves a wire address. A named face (`ID@face`) is read with
// [Resolver.Ref]; a bare id is resolved in w with [Resolver.InWorld]. An
// address that does not parse is a miss.
func (r *Resolver) Address(ctx context.Context, w World, entityType, addr string) (Resolved, bool, error) {
	ref, err := entity.ParseRef(addr)
	if err != nil {
		// An address that cannot name a row names none: the same miss.
		return Resolved{}, false, nil //nolint:nilerr // a malformed address is a miss, not a fault
	}
	if ref.Face.IsImplicit() {
		return r.InWorld(ctx, w, entityType, ref.ID)
	}
	return r.Ref(ctx, w, entityType, ref)
}

// Ref reads the row ref names, literally, whatever world w resolves to. The
// world still matters in one way: a denied world blocks the read, so a
// principal cannot bypass a world grant by spelling the face.
func (r *Resolver) Ref(ctx context.Context, w World, entityType string, ref entity.Ref) (Resolved, bool, error) {
	faces, ok, err := r.admit(ctx, w, entityType, ref.ID)
	if err != nil || !ok {
		return Resolved{}, false, err
	}
	if !faces.Contains(ref.Face) {
		return Resolved{}, false, nil
	}
	e, ok := r.loadRef(ctx, entityType, ref)
	if !ok {
		return Resolved{}, false, nil
	}
	return r.serve(ctx, w, entityType, faces, e)
}

// InWorld resolves a bare id to the face w selects for it.
//
// The default world reads the implicit face "". A faced type has no row
// there, so it misses until the default world is generated (TKT-7IZHP0).
// Any other world reads through one ListEntities query carrying the world
// and the faces of id the principal may read: the ACL trims the candidates
// first and the world ranks what is left, exactly as the list path does.
func (r *Resolver) InWorld(ctx context.Context, w World, entityType, id string) (Resolved, bool, error) {
	if !w.denied && !w.scope.IsSet() {
		return Resolved{}, false, fmt.Errorf("%w: visibility: InWorld with an unset world (use WorldOf)",
			store.ErrInvalidQuery)
	}
	faces, ok, err := r.admit(ctx, w, entityType, id)
	if err != nil || !ok {
		return Resolved{}, false, err
	}
	var e *entity.Entity
	if w.scope.IsTrivial() {
		if !faces.Contains("") {
			return Resolved{}, false, nil
		}
		// The implicit face "" is the default world's answer for a bare id
		// until TKT-7IZHP0 generates a default world for faced types; that
		// ticket removes this read.
		e, ok = r.loadRef(ctx, entityType, entity.Ref{ID: id})
	} else {
		e, ok = r.loadInWorld(ctx, w.scope, entityType, id, faces)
	}
	if !ok {
		return Resolved{}, false, nil
	}
	return r.serve(ctx, w, entityType, faces, e)
}

// Family answers which faces of id the principal may read, reading headers
// only. It takes no world: it serves entity-level questions (does a readable
// face of this id exist, of this type), and the routes that ask them refuse
// a non-default world.
//
// Every stored face must have the claimed type, else the family is a miss.
// A face the principal may not read is left out and never counted, so its
// existence stays hidden; a family with no readable face is a miss. A caller
// that needs a face's content reads it with [Resolver.Ref].
func (r *Resolver) Family(ctx context.Context, entityType, id string) (Family, bool, error) {
	faces, ok, err := r.admit(ctx, World{}, entityType, id)
	if err != nil || !ok {
		return Family{}, false, err
	}
	headers, ok := r.headersOf(ctx, entityType, id)
	if !ok {
		return Family{}, false, nil
	}
	return r.familyOf(entityType, id, faces, headers)
}

// headersOf reads every stored face header of id. A failed read is logged
// and answered as a miss, like every other resolver load (RR-FE1EGP).
func (r *Resolver) headersOf(ctx context.Context, entityType, id string) ([]store.EntityHeader, bool) {
	out, err := store.FamilyHeaders(ctx, r.load, id)
	if err != nil {
		warnLoad("family", entityType, id, err)
		return nil, false
	}
	return out, true
}

// familyOf keeps the headers whose face is in faces. Every header must have
// entityType, else the family is a miss.
func (r *Resolver) familyOf(entityType, id string, faces FaceSet, headers []store.EntityHeader) (Family, bool, error) {
	var readable []entity.Face
	for _, h := range headers {
		if h.Type != entityType {
			return Family{}, false, nil
		}
		if faces.Contains(h.Face) {
			readable = append(readable, h.Face)
		}
	}
	if len(readable) == 0 {
		return Family{}, false, nil
	}
	r.sortFaces(entityType, readable)
	return Family{ID: id, Type: entityType, Faces: readable}, true, nil
}

// sortFaces orders faces as [WithFaceOrder] documents.
func (r *Resolver) sortFaces(entityType string, faces []entity.Face) {
	if r.faceOrder == nil {
		slices.Sort(faces)
		return
	}
	// A linear scan, not a rank map: a type declares a handful of faces, so
	// the scan is cheaper than building a map on every call (RR-TLQPK6).
	order := r.faceOrder(entityType)
	undeclared := len(order) + 1
	key := func(f entity.Face) int {
		if f.IsImplicit() {
			return 0
		}
		if i := slices.Index(order, string(f)); i >= 0 {
			return i + 1
		}
		return undeclared
	}
	slices.SortFunc(faces, func(a, b entity.Face) int {
		if d := cmp.Compare(key(a), key(b)); d != 0 {
			return d
		}
		return cmp.Compare(a, b)
	})
}

// admit applies the gates that run before any load: the world, then the
// faces of id whose row passes the read verdict, within the readable faces
// of the type. The answer is per face, so a verdict that holds on one face
// row and not on another admits only the first.
func (r *Resolver) admit(ctx context.Context, w World, entityType, id string) (FaceSet, bool, error) {
	if w.denied || id == "" {
		return FaceSet{}, false, nil
	}
	verdicts, err := r.gate.ReadableFacesMany(ctx, entityType, []string{id})
	if err != nil {
		return FaceSet{}, false, err
	}
	faces := VerdictSet(verdicts.For(id))
	if faces.IsNone() {
		return FaceSet{}, false, nil
	}
	typeFaces, err := ReadableFaces(ctx, r.gate, entityType)
	if err != nil {
		return FaceSet{}, false, err
	}
	faces = faces.Intersect(typeFaces)
	if faces.IsNone() {
		return FaceSet{}, false, nil
	}
	return faces, true, nil
}

// loadRef reads one row by its address.
func (r *Resolver) loadRef(ctx context.Context, entityType string, ref entity.Ref) (*entity.Entity, bool) {
	e, err := r.load.GetEntity(ctx, entity.Ref{ID: ref.ID, Face: ref.Face})
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			warnLoad("ref", entityType, ref.String(), err)
		}
		return nil, false
	}
	return e, e != nil
}

// loadInWorld reads the face scope selects for id among faces.
func (r *Resolver) loadInWorld(
	ctx context.Context, scope store.WorldScope, entityType, id string, faces FaceSet,
) (*entity.Entity, bool) {
	faceIn, ok := faces.QueryFaces()
	if !ok {
		return nil, false
	}
	q := store.EntityQuery{IDs: []string{id}, Faces: store.InWorld(scope), FaceIn: faceIn}
	for e, err := range r.load.ListEntities(ctx, q) {
		if err != nil {
			warnLoad("world", entityType, id, err)
			return nil, false
		}
		if e != nil && e.ID == id {
			return e, true
		}
	}
	return nil, false
}

// serve applies the gates that need the loaded row, then redacts it once.
func (r *Resolver) serve(
	ctx context.Context, w World, entityType string, faces FaceSet, e *entity.Entity,
) (Resolved, bool, error) {
	if e.Type != entityType || !faces.Contains(e.Face) {
		return Resolved{}, false, nil
	}
	ctx = PrimeTraversals(ctx, r.redact, []*entity.Entity{e})
	via, pos := w.scope.RuleAt(entityType, e.Face)
	return Resolved{Entity: Redact(ctx, r.redact, e), Via: via, ChainPosition: pos}, true, nil
}

// warnLoad records a load failure that the resolver answers as a miss. It
// names the address (an id, or `ID@face`) and never a property value.
func warnLoad(mode, entityType, addr string, err error) {
	slog.Warn("visibility: resolver load failed; answering not-found",
		"mode", mode, "type", entityType, "addr", addr, "err", err)
}
