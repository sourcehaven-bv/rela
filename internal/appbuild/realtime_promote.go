package appbuild

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Warning codes of a promote that created its entity but not everything
// around it.
const (
	// warnPromoteRelateFailed: the relation to the anchor was not written.
	// Only the filesystem tier reports it; elsewhere the promote fails.
	warnPromoteRelateFailed = "promote_relate_failed"
	// warnPromoteFollowUpFailed: an automation or the cascade after the
	// create failed. The entity stays.
	warnPromoteFollowUpFailed = "promote_followup_failed"
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
			Code: warnPromoteRelateFailed, Detail: "created " + created.ID + " without its " + r.Relate + " relation",
		})
	}
	if res.FollowUpErr != nil {
		out.Warnings = append(out.Warnings, entity.Warning{
			Code: warnPromoteFollowUpFailed, Detail: "created " + created.ID + "; a step after the create failed",
		})
	}
	return out, nil
}

// promoteError turns a [entitymanager.CreateLinked] error into the error
// the service answers. It wraps [realtime.ErrNothingWritten] when nothing
// was written: always when the promote was atomic, and otherwise when the
// error is a refused check or a refusal of the entity write itself.
func promoteError(err error, atomic bool) error {
	d := classifyPromote(err)
	nothing := atomic || entitymanager.WroteNothing(err) || refusedWrite(err)
	var out error = d
	if d == nil {
		out = err
	}
	if nothing {
		return fmt.Errorf("%w: %w", realtime.ErrNothingWritten, out)
	}
	return out
}

// classifyPromote maps a create error to a caller-facing refusal, with the
// classes the data-entry create route answers. An error it does not know
// is a server error, and it returns nil.
func classifyPromote(err error) *realtime.DenialError {
	var forbidden *acl.ForbiddenError
	var fieldWrite *affordances.FieldWriteError
	var invalid *entitymanager.ValidationError
	var invalidRel *entitymanager.InvalidRelationError
	switch {
	case errors.As(err, &forbidden):
		return &realtime.DenialError{
			Status: http.StatusForbidden, Type: realtime.TypePromoteDenied,
			Title: "the promote's entity create is refused", Rule: forbidden.Decision.RuleID,
		}
	case errors.As(err, &fieldWrite):
		return &realtime.DenialError{
			Status: http.StatusForbidden, Type: realtime.TypePromoteDenied,
			Title: "the promote writes a field the caller may not write", Rule: fieldWrite.RuleID(),
		}
	case errors.Is(err, statemachine.ErrGuardDenied):
		return &realtime.DenialError{
			Status: http.StatusForbidden, Type: realtime.TypePromoteDenied,
			Title: "the promote's entity create is refused by its state machine",
		}
	case errors.Is(err, entitymanager.ErrEntityAlreadyExists), errors.Is(err, entitymanager.ErrRelationAlreadyExists):
		return &realtime.DenialError{
			Status: http.StatusConflict, Type: realtime.TypePromoteConflict, Title: err.Error(),
		}
	case errors.As(err, &invalid), errors.As(err, &invalidRel),
		errors.Is(err, entitymanager.ErrFaceRequired), errors.Is(err, entitymanager.ErrFaceNotDeclared),
		errors.Is(err, entitymanager.ErrEntityNotFound):
		return &realtime.DenialError{
			Status: http.StatusUnprocessableEntity, Type: realtime.TypePromoteInvalid, Title: err.Error(),
		}
	}
	return nil
}

// refusedWrite reports whether err is a refusal of the entity write that
// leaves nothing written: validation, including a unique collision, or an
// existing id.
func refusedWrite(err error) bool {
	var invalid *entitymanager.ValidationError
	return errors.As(err, &invalid) || errors.Is(err, entitymanager.ErrEntityAlreadyExists)
}
