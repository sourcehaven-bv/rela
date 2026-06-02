package cli

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ShowCmd prints an entity and its incoming/outgoing relations.
type ShowCmd struct {
	ID string `arg:"" help:"Entity ID (e.g. REQ-001)."`
}

// Run dispatches `rela show <id>`.
func (c *ShowCmd) Run(ctx context.Context, svc *cliServices) error {
	st := svc.Store()

	e, err := st.GetEntity(ctx, c.ID)
	if err != nil {
		return classifyReadError(c.ID, err)
	}

	var incoming, outgoing []*entity.Relation
	inQ := store.RelationQuery{EntityID: c.ID, Direction: store.DirectionIncoming}
	for r, err := range st.ListRelations(ctx, inQ) {
		if err != nil {
			break
		}
		incoming = append(incoming, r)
	}
	outQ := store.RelationQuery{EntityID: c.ID, Direction: store.DirectionOutgoing}
	for r, err := range st.ListRelations(ctx, outQ) {
		if err != nil {
			break
		}
		outgoing = append(outgoing, r)
	}

	return out.WriteEntity(e, incoming, outgoing)
}

// entityNotFoundError is the typed "entity not found" error used by
// show / delete / update and any other read path that wants to
// distinguish missing-entity from other read failures.
type entityNotFoundError struct {
	ID string
}

func (e *entityNotFoundError) Error() string {
	return "entity not found: " + e.ID
}

// classifyReadError maps a store.GetEntity error onto a user-facing
// type. Today it only distinguishes ErrNotFound from every other
// error shape, but the indirection stays so future classes plug in
// without touching every caller.
func classifyReadError(id string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return &entityNotFoundError{ID: id}
	}
	return err
}
