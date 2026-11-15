package mapimport

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Writer is the write path an import applies through. entitymanager.Manager
// satisfies it, so validation, automations, audit and the write ACL apply to
// every change.
type Writer interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
	PatchEntity(ctx context.Context, id string, p entity.Patch) (*entity.UpdateResult, error)
	CreateRelation(ctx context.Context, key entity.RelationKey, opts entity.RelationOptions) (*entity.Relation, error)
	DeleteRelation(ctx context.Context, key entity.RelationKey) error
}

// ApplyOptions tune [Apply].
type ApplyOptions struct {
	// AllowErrors applies the rest of a plan that has record errors. The
	// records with errors are left out.
	AllowErrors bool
}

// Result counts what [Apply] wrote and lists what failed.
type Result struct {
	Created          int       `json:"created"`
	Updated          int       `json:"updated"`
	RelationsAdded   int       `json:"relations_added"`
	RelationsRemoved int       `json:"relations_removed"`
	Failures         []Failure `json:"failures,omitempty"`
}

// Failure is one write the manager refused.
type Failure struct {
	What  string `json:"what"`
	Error string `json:"error"`
}

// ErrPlanHasErrors is returned by [Apply] for a plan with record errors when
// ApplyOptions.AllowErrors is off.
var ErrPlanHasErrors = errors.New("the plan has record errors; fix them or allow them")

// Apply writes the plan: creates, then updates, then relation removals, then
// relation adds. A refused write is a Failure and the rest goes on; a
// relation to an entity whose create failed is a Failure too. Nil: w is
// rejected.
func Apply(ctx context.Context, w Writer, p *Plan, opts ApplyOptions) (*Result, error) {
	if w == nil || p == nil {
		return nil, errors.New("mapimport: a writer and a plan are required")
	}
	if len(p.Errors) > 0 && !opts.AllowErrors {
		return nil, ErrPlanHasErrors
	}
	res := &Result{}
	failed := map[*createOp]bool{}
	for _, op := range p.creates {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		e := *op.e
		out, err := w.CreateEntity(ctx, &e, op.opts)
		if err != nil {
			failed[op] = true
			res.Failures = append(res.Failures, Failure{What: "create " + op.label, Error: err.Error()})
			continue
		}
		op.id = out.Entity.ID
		res.Created++
	}
	for _, op := range p.patches {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if _, err := w.PatchEntity(ctx, op.id, op.patch); err != nil {
			res.Failures = append(res.Failures, Failure{What: "update " + op.id, Error: err.Error()})
			continue
		}
		res.Updated++
	}
	for _, key := range p.removes {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if err := w.DeleteRelation(ctx, key); err != nil {
			res.Failures = append(res.Failures, Failure{What: "remove " + relLabel(key), Error: err.Error()})
			continue
		}
		res.RelationsRemoved++
	}
	for _, op := range p.adds {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		key := entity.RelationKey{From: op.from.endID(), Type: op.typ, To: op.to.endID()}
		if failed[op.from.new] || failed[op.to.new] {
			res.Failures = append(res.Failures, Failure{
				What:  fmt.Sprintf("add %s %s -> %s", op.typ, op.from.label(), op.to.label()),
				Error: "an endpoint was not created",
			})
			continue
		}
		if _, err := w.CreateRelation(ctx, key, entity.RelationOptions{}); err != nil {
			res.Failures = append(res.Failures, Failure{What: "add " + relLabel(key), Error: err.Error()})
			continue
		}
		res.RelationsAdded++
	}
	return res, nil
}

func relLabel(k entity.RelationKey) string { return fmt.Sprintf("%s %s -> %s", k.Type, k.From, k.To) }
