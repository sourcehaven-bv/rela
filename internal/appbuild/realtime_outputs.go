package appbuild

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// managerOutputs is the [realtime.OutputWriter] over the entity manager
// (TKT-98TR9A). It writes as the poster, with tool `realtime`, so the ACL,
// the field grants, validation, automations and the graph audit apply as
// they do to any write, and the audit tells a session output apart from
// the person's own write.
//
// A set goes through the field-gated [entitymanager.Manager.PatchEntity]:
// every property, literal or authored, passes the field write gate, and
// the row ACL is asked again on the row as it stands at the write. A
// property the poster cannot see is never erased. A create goes through
// [entitymanager.CreateLinked]; where join is set, the progress write
// commits in the create's transaction, as a promote's mark does.
type managerOutputs struct {
	gated *entitymanager.Manager
	st    store.Store
	meta  *metamodel.Metamodel
	join  joinChannels
	// policy is the policy the manager evaluates; nil when none is
	// configured.
	policy *acl.Declarative
	// bind opens a Request for the principal on ctx on policy.
	bind func(context.Context) (context.Context, error)
}

// errOutputsNoPoster refuses a write as a poster who cannot be rebuilt.
var errOutputsNoPoster = errors.New("appbuild: outputs: the poster no longer resolves")

// newManagerOutputs builds the writer over mgr, st and meta. join is what
// [txChannelsFor] returned; d is the policy mgr evaluates.
//
// Nil: mgr, st and meta are rejected. join is accepted and means a create
// is not atomic. d is accepted and means no policy is configured.
func newManagerOutputs(
	mgr *entitymanager.Manager, st store.Store, meta *metamodel.Metamodel, join joinChannels, d *acl.Declarative,
) (managerOutputs, error) {
	if mgr == nil || st == nil || meta == nil {
		return managerOutputs{}, errors.New("appbuild: outputs: entity manager, store and metamodel are required")
	}
	return managerOutputs{
		gated: entitymanager.FieldGated(mgr), st: st, meta: meta, join: join, policy: d, bind: identityBinderFor(d),
	}, nil
}

// as stamps ctx with the writer. Inline, that is the request's principal
// with tool realtime, and the request's own Request retooled: it keeps the
// unmatched-verified mark, so `unmatched_principal: reject` still applies.
// On a retry, see [managerOutputs.asRetry].
func (o managerOutputs) as(ctx context.Context, retry *realtime.OutputIdentity) (context.Context, error) {
	if retry != nil {
		return o.asRetry(ctx, retry)
	}
	p := principal.From(ctx)
	p.Tool = principal.ToolRealtime
	if o.policy == nil {
		return principal.With(ctx, p), nil
	}
	req, err := o.policy.RequestFrom(ctx)
	switch {
	case err == nil:
		retooled := acl.RetoolRequest(req, principal.ToolRealtime)
		return acl.WithRequest(principal.With(ctx, retooled.Principal()), retooled), nil
	case errors.Is(err, acl.ErrNoIdentity):
		// A transport that attached no Request: open one, as it would.
		return o.bind(principal.With(ctx, p))
	default:
		return ctx, err
	}
}

// asRetry rebuilds the poster from the mark, with the role claims verified
// at the post. A poster who named a user entity at the post fails when the
// entity is gone. A verified poster who matched none is resolved again
// and marked verified, as the data-entry edge does, so
// `unmatched_principal` judges the retry as it judged the post.
func (o managerOutputs) asRetry(ctx context.Context, id *realtime.OutputIdentity) (context.Context, error) {
	p := principal.VerifiedFrom(id.Writer.User, principal.ToolRealtime, principal.Claims{
		PrincipalType: id.Writer.PrincipalType, OrgID: id.Writer.OrgID, Roles: id.Roles,
	})
	ctx = principal.With(ctx, p)
	if o.policy == nil {
		return ctx, nil
	}
	if id.UserEntity {
		exists, err := o.userExists(ctx, p.User)
		if err != nil {
			return ctx, err
		}
		if !exists {
			return ctx, errOutputsNoPoster
		}
	}
	if !id.Unmatched {
		return o.bind(ctx)
	}
	ident, err := o.policy.IdentityResolver().Resolve(ctx, p)
	if err != nil {
		return ctx, err
	}
	req, err := o.policy.ForIdentity(ident.WithVerifiedAssertion())
	if err != nil {
		return ctx, err
	}
	return acl.WithRequest(principal.With(ctx, ident.Principal()), req), nil
}

// userExists reports whether user names an entity of the policy's user
// entity type. Without one, no poster names a user entity.
func (o managerOutputs) userExists(ctx context.Context, user string) (bool, error) {
	typ := o.policy.Policy().UserEntityType
	if typ == "" || user == "" {
		return false, nil
	}
	faces := metamodel.FaceOrderOf(o.meta, typ)
	if len(faces) == 0 {
		faces = []string{""}
	}
	for _, face := range faces {
		addr := user
		if face != "" {
			addr += "@" + face
		}
		ref, perr := entity.ParseRef(addr)
		if perr != nil {
			continue // a name that is no address names no entity
		}
		e, err := o.st.GetEntity(ctx, ref)
		switch {
		case err == nil:
			return e.Type == typ, nil
		case !errors.Is(err, store.ErrNotFound):
			return false, err
		}
	}
	return false, nil
}

// Poster reports the identity facts of the caller on ctx.
func (o managerOutputs) Poster(ctx context.Context) (realtime.OutputPoster, error) {
	var out realtime.OutputPoster
	if o.policy == nil {
		return out, nil
	}
	if req, err := o.policy.RequestFrom(ctx); err == nil {
		out.Unmatched = req.UnmatchedVerified()
	}
	exists, err := o.userExists(ctx, principal.From(ctx).User)
	if err != nil {
		return out, err
	}
	out.UserEntity = exists
	return out, nil
}

// Check asks t's write questions as the poster, without writing. A set
// runs [entitymanager.Manager.AuthorizeUpdate], read gate first; the write
// authorizes again. A create probes the create and the relation questions
// without an audit row.
func (o managerOutputs) Check(ctx context.Context, t realtime.OutputTarget) error {
	ctx, err := o.as(ctx, nil)
	if err != nil {
		if errors.Is(err, acl.ErrUnstampedPrincipal) {
			return outputCheckDenial(nil)
		}
		return err
	}
	if t.Kind == realtimecfg.OutputSet {
		row, err := o.setRow(ctx, t)
		if err != nil {
			return outputCheckError(err)
		}
		if _, err := o.gated.AuthorizeUpdate(ctx, row); err != nil {
			return outputCheckError(err)
		}
		return nil
	}
	if err := entitymanager.CheckCreate(ctx, o.gated, t.Type, t.Face); err != nil {
		return outputCheckError(err)
	}
	if t.Relate != "" {
		if err := entitymanager.CheckRelate(ctx, o.gated, t.Relate, t.Type, t.Face); err != nil {
			return outputCheckError(err)
		}
	}
	return nil
}

// Write writes one output as the poster.
func (o managerOutputs) Write(ctx context.Context, r realtime.OutputRequest) (realtime.OutputWritten, error) {
	ctx, err := o.as(ctx, r.Retry)
	if err != nil {
		if errors.Is(err, errOutputsNoPoster) || errors.Is(err, acl.ErrUnstampedPrincipal) {
			return realtime.OutputWritten{}, &realtime.OutputError{
				Code: realtime.OutputCodeForbidden, WroteNothing: true,
			}
		}
		return realtime.OutputWritten{}, err
	}
	ctx = audit.WithTriggeredBy(ctx, r.Origin)
	if r.Target.Kind == realtimecfg.OutputSet {
		return o.set(ctx, r)
	}
	return o.create(ctx, r)
}

// setRow reads the raw row a set writes. Whether it has a face is the
// anchor type's declaration: a faceless type is read without one, and a
// faced type only at the target's face, never as a bare id. The row is
// write-prep: AuthorizeUpdate and PatchEntity gate it.
func (o managerOutputs) setRow(ctx context.Context, t realtime.OutputTarget) (*entity.Entity, error) {
	addr := t.AnchorID
	if len(metamodel.FaceOrderOf(o.meta, t.AnchorType)) > 0 {
		if t.Face == "" {
			return nil, entitymanager.ErrEntityNotFound
		}
		addr += "@" + string(t.Face)
	}
	ref, err := entity.ParseRef(addr)
	if err != nil {
		return nil, entitymanager.ErrEntityNotFound
	}
	row, err := o.st.GetEntity(ctx, ref)
	if errors.Is(err, store.ErrNotFound) || (err == nil && row.Type != t.AnchorType) {
		return nil, entitymanager.ErrEntityNotFound
	}
	return row, err
}

// set patches the anchor with r's properties through the field-gated
// manager, which asks the row ACL and the field gate on the current row.
func (o managerOutputs) set(ctx context.Context, r realtime.OutputRequest) (realtime.OutputWritten, error) {
	row, err := o.setRow(ctx, r.Target)
	if err != nil {
		return realtime.OutputWritten{}, patchDenial(err)
	}
	res, err := o.gated.PatchEntity(ctx, row.Ref().String(), entity.Patch{Properties: r.Props})
	if err != nil {
		return realtime.OutputWritten{}, patchDenial(err)
	}
	return realtime.OutputWritten{Warnings: warningCodes(res.Warnings)}, nil
}

// create makes the entity r names, related to the anchor. Where the stores
// join, the progress write commits with it. Otherwise r.Started records
// the start first, so a crash after the create is never repeated.
func (o managerOutputs) create(ctx context.Context, r realtime.OutputRequest) (realtime.OutputWritten, error) {
	atomic := o.join != nil
	lc := entitymanager.LinkedCreate{
		Entity:           &entity.Entity{Type: r.Target.Type, Properties: r.Props},
		Options:          entity.CreateOptions{Face: r.Target.Face},
		Authored:         r.Authored,
		RelateBestEffort: !atomic,
	}
	if r.Target.Relate != "" {
		lc.RelateType, lc.RelateTo = r.Target.Relate, r.Target.AnchorID
	}
	var marked channels.Message
	if atomic {
		lc.InTx = func(ctx context.Context, view store.Store, created *entity.Entity) error {
			cs, err := o.join(view)
			if err != nil {
				return err
			}
			marked, err = r.Progress.Apply(ctx, cs, created.ID, created.Type)
			return err
		}
	} else if r.Started != nil {
		if err := r.Started(ctx); err != nil {
			return realtime.OutputWritten{}, &realtime.OutputError{
				Code: realtime.OutputCodeError, Retryable: true, WroteNothing: true,
			}
		}
	}
	res, err := entitymanager.CreateLinked(ctx, o.gated, lc)
	if err != nil {
		return realtime.OutputWritten{}, createFailure(err, atomic || entitymanager.WroteNothing(err))
	}
	created := res.Create.Entity
	out := realtime.OutputWritten{
		Entity:   &channels.Promoted{ID: created.ID, Type: created.Type},
		Warnings: warningCodes(res.Create.Warnings),
		Marked:   atomic, Message: marked,
	}
	if res.RelateErr != nil {
		out.Warnings = append(out.Warnings, realtime.WarnPromoteRelateFailed)
	}
	if res.FollowUpErr != nil {
		out.Warnings = append(out.Warnings, realtime.WarnPromoteFollowUpFailed)
	}
	return out, nil
}

// outputCheckError is the pre-check's refusal for err, or err itself for a
// server error. A row the poster cannot read is refused like a denied one:
// the 403 names the output, not whether the row exists.
func outputCheckError(err error) error {
	if errors.Is(err, entitymanager.ErrEntityNotFound) {
		return outputCheckDenial(nil)
	}
	switch entitymanager.ClassifyCreateError(err) {
	case entitymanager.CreateErrForbidden, entitymanager.CreateErrFace, entitymanager.CreateErrInvalid:
		return outputCheckDenial(err)
	case entitymanager.CreateErrConflict, entitymanager.CreateErrUnknown:
	}
	return err
}

// outputCheckDenial is the pre-check's refusal. Its title never quotes
// err; the rule is the ACL rule that denied, when there is one.
func outputCheckDenial(err error) *realtime.DenialError {
	d := &realtime.DenialError{
		Status: http.StatusForbidden, Type: realtime.TypeSessionOutputDenied, Title: "the write is refused",
	}
	if forbidden, ok := errors.AsType[*acl.ForbiddenError](err); ok {
		d.Rule = forbidden.Decision.RuleID
	}
	if fieldWrite, ok := errors.AsType[*affordances.FieldWriteError](err); ok {
		d.Title, d.Rule = "a field the poster may not write", fieldWrite.RuleID()
	}
	return d
}

// patchDenial classifies a set's error. A lost version race is retried;
// any other refusal is final. No code quotes the error or an id.
func patchDenial(err error) error {
	var conflict *store.VersionConflictError
	switch {
	case errors.Is(err, entitymanager.ErrEntityNotFound):
		return &realtime.OutputError{Code: realtime.OutputCodeNotFound, WroteNothing: true}
	case entitymanager.IsUniqueViolation(err):
		return &realtime.OutputError{Code: realtime.OutputCodeConflict, WroteNothing: true}
	case errors.As(err, &conflict):
		return &realtime.OutputError{Code: realtime.OutputCodeConflict, Retryable: true, WroteNothing: true}
	case errors.Is(err, statemachine.ErrIllegalTransition):
		// Only an update moves a state property, so the create classifier
		// does not know this refusal.
		return &realtime.OutputError{Code: realtime.OutputCodeInvalid, WroteNothing: true}
	}
	if f := classifiedFailure(err); f != nil {
		f.WroteNothing = true
		return f
	}
	return err
}

// createFailure classifies a create's error, as the promote classifier
// does. wroteNothing says the create left nothing behind.
func createFailure(err error, wroteNothing bool) error {
	if f := classifiedFailure(err); f != nil {
		f.WroteNothing = wroteNothing
		return f
	}
	if entitymanager.IsUniqueViolation(err) {
		return &realtime.OutputError{Code: realtime.OutputCodeConflict, WroteNothing: wroteNothing}
	}
	if wroteNothing {
		return &realtime.OutputError{Code: realtime.OutputCodeError, Retryable: true, WroteNothing: true}
	}
	return err
}

// classifiedFailure is the final failure of a refusal both writes share,
// or nil for an error that is not one.
func classifiedFailure(err error) *realtime.OutputError {
	if _, ok := errors.AsType[*affordances.FieldWriteError](err); ok {
		return &realtime.OutputError{Code: realtime.OutputCodeFieldDenied}
	}
	if entitymanager.IsUniqueViolation(err) {
		return &realtime.OutputError{Code: realtime.OutputCodeConflict}
	}
	switch entitymanager.ClassifyCreateError(err) {
	case entitymanager.CreateErrForbidden:
		return &realtime.OutputError{Code: realtime.OutputCodeForbidden}
	case entitymanager.CreateErrConflict:
		return &realtime.OutputError{Code: realtime.OutputCodeConflict}
	case entitymanager.CreateErrFace:
		return &realtime.OutputError{Code: realtime.OutputCodeInvalid}
	case entitymanager.CreateErrInvalid:
		if errors.Is(err, entitymanager.ErrEntityNotFound) {
			return &realtime.OutputError{Code: realtime.OutputCodeNotFound}
		}
		return &realtime.OutputError{Code: realtime.OutputCodeInvalid, Props: invalidProps(err)}
	case entitymanager.CreateErrUnknown:
	}
	return nil
}

// invalidProps names the properties a validation error refuses.
func invalidProps(err error) []string {
	invalid, ok := errors.AsType[*entitymanager.ValidationError](err)
	if !ok {
		return nil
	}
	var props []string
	for _, e := range invalid.Errors {
		if e.Property != "" {
			props = append(props, e.Property)
		}
	}
	slices.Sort(props)
	return slices.Compact(props)
}

// warningCodes are the codes of ws.
func warningCodes(ws []entity.Warning) []string {
	if len(ws) == 0 {
		return nil
	}
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Code)
	}
	return out
}
