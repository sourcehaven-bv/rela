package appbuild

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// joinChannels returns the channel store that writes in the transaction
// of view. Nil means the channel store cannot join the entity store's
// transaction, so the promote is not atomic.
type joinChannels func(view store.Store) (channels.Store, error)

// managerPromoter is the [realtime.Promoter] over the entity manager. It
// creates the entity as the caller, through the field-gated manager, so the
// create ACL, the field grants, validation, automations and the audit log
// apply as they do to a create over HTTP.
//
// On postgres and sqlite, join is set: the entity, the relation to the
// anchor and the action mark commit in one transaction, and any error
// leaves none of them. On the other tiers join is nil: the entity is
// written first, a failed relation is a warning, and the service marks the
// action afterwards.
type managerPromoter struct {
	mgr  *entitymanager.Manager
	join joinChannels
}

// newManagerPromoter builds the promoter over mgr. join is what
// [txChannelsFor] returned for the stores.
//
// Nil: mgr is rejected. join is accepted and means the promote is not
// atomic.
func newManagerPromoter(mgr *entitymanager.Manager, join joinChannels) (managerPromoter, error) {
	if mgr == nil {
		return managerPromoter{}, errors.New("appbuild: promoter: entity manager is nil")
	}
	return managerPromoter{mgr: entitymanager.FieldGated(mgr), join: join}, nil
}

// Promote creates the entity r asks for.
func (p managerPromoter) Promote(ctx context.Context, r realtime.PromoteRequest) (realtime.PromoteResult, error) {
	ctx = audit.WithTriggeredBy(ctx, r.Origin)
	atomic := p.join != nil
	lc := entitymanager.LinkedCreate{
		Entity:           &entity.Entity{Type: r.Type, Properties: r.Props},
		Options:          entity.CreateOptions{Face: r.Face},
		RelateType:       r.Relate,
		RelateTo:         r.Anchor.ID,
		RelateBestEffort: !atomic,
	}
	var marked channels.Message
	if atomic {
		lc.InTx = func(ctx context.Context, view store.Store, created *entity.Entity) error {
			cs, err := p.join(view)
			if err != nil {
				return err
			}
			marked, err = r.Mark.Apply(ctx, cs, created.ID, created.Type)
			return err
		}
	}
	res, err := entitymanager.CreateLinked(ctx, p.mgr, lc)
	if err != nil {
		return realtime.PromoteResult{}, promoteError(err, atomic)
	}
	created := res.Create.Entity
	out := realtime.PromoteResult{
		ID: created.ID, Type: created.Type, Warnings: res.Create.Warnings, Marked: atomic, Message: marked,
	}
	if res.RelateErr != nil {
		out.Warnings = append(out.Warnings, entity.Warning{
			Code: realtime.WarnPromoteRelateFailed, Detail: "the entity was created without its " + r.Relate + " relation",
		})
	}
	if res.FollowUpErr != nil {
		out.Warnings = append(out.Warnings, entity.Warning{
			Code: realtime.WarnPromoteFollowUpFailed, Detail: "the entity was created; a step after the create failed",
		})
	}
	return out, nil
}

// CheckCreate asks the manager's create ACL question as a probe; see
// [entitymanager.CheckCreate].
func (p managerPromoter) CheckCreate(ctx context.Context, typ string, face entity.Face) error {
	err := entitymanager.CheckCreate(ctx, p.mgr, typ, face)
	if err == nil {
		return nil
	}
	if d := classifyPromote(err); d != nil {
		return d
	}
	return err
}

// promoteError turns a [entitymanager.CreateLinked] error into the error
// the service answers. It wraps [realtime.ErrNothingWritten] when nothing
// was written: always when the promote was atomic, and otherwise when
// [entitymanager.WroteNothing] says so.
func promoteError(err error, atomic bool) error {
	d := classifyPromote(err)
	var out error = d
	if d == nil {
		out = err
	}
	if atomic || entitymanager.WroteNothing(err) {
		return fmt.Errorf("%w: %w", realtime.ErrNothingWritten, out)
	}
	return out
}

// classifyPromote maps a create error to a caller-facing refusal through
// the classifier the HTTP create route uses. An error that is not a
// refusal is a server error, and it returns nil.
func classifyPromote(err error) *realtime.DenialError {
	return promoteDenial(entitymanager.ClassifyCreateError(err), err)
}

// promoteDenial is the refusal of class. Its title never quotes err: a
// create error can name entity ids, such as the holder of a unique value,
// that the caller may not read. Validation names the failing properties
// only.
func promoteDenial(class entitymanager.CreateErrorClass, err error) *realtime.DenialError {
	switch class {
	case entitymanager.CreateErrForbidden:
		d := &realtime.DenialError{
			Status: http.StatusForbidden, Type: realtime.TypePromoteDenied,
			Title: "the promote's entity create is refused",
		}
		if forbidden, ok := errors.AsType[*acl.ForbiddenError](err); ok {
			d.Rule = forbidden.Decision.RuleID
		}
		if fieldWrite, ok := errors.AsType[*affordances.FieldWriteError](err); ok {
			d.Title, d.Rule = "the promote writes a field the caller may not write", fieldWrite.RuleID()
		}
		return d
	case entitymanager.CreateErrConflict:
		return &realtime.DenialError{
			Status: http.StatusConflict, Type: realtime.TypePromoteConflict,
			Title: "the promoted entity or its relation already exists",
		}
	case entitymanager.CreateErrFace:
		return &realtime.DenialError{
			Status: http.StatusUnprocessableEntity, Type: realtime.TypePromoteInvalid,
			Title: "the promote names a face its entity type does not admit",
		}
	case entitymanager.CreateErrInvalid:
		return &realtime.DenialError{
			Status: http.StatusUnprocessableEntity, Type: realtime.TypePromoteInvalid, Title: invalidTitle(err),
		}
	case entitymanager.CreateErrUnknown:
	}
	return nil
}

// invalidTitle names the properties a validation error refuses, and
// nothing else of the error.
func invalidTitle(err error) string {
	const base = "the promoted entity is refused by the schema"
	invalid, ok := errors.AsType[*entitymanager.ValidationError](err)
	if !ok {
		return base
	}
	var props []string
	for _, e := range invalid.Errors {
		if e.Property != "" {
			props = append(props, e.Property+" ("+string(e.Type)+")")
		}
	}
	if len(props) == 0 {
		return base
	}
	slices.Sort(props)
	return base + ": " + strings.Join(slices.Compact(props), ", ")
}
