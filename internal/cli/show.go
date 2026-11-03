package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// ShowCmd prints an entity, its incoming/outgoing relations, and the
// entities whose body mentions it.
type ShowCmd struct {
	ID string `arg:"" help:"Entity address: ID, or ID@face for a type with faces (e.g. REQ-001, DOC-1@draft)."`
}

// Run dispatches `rela show <id>`.
func (c *ShowCmd) Run(ctx context.Context, svc *readServices) error {
	st := svc.Store

	e, err := readAddress(ctx, st, svc.Families, svc.World, c.ID)
	if err != nil {
		return classifyReadError(c.ID, err)
	}

	// Relations are keyed on the bare id, so query by the row that was read,
	// not by the address the user typed.
	var incoming, outgoing []*entity.Relation
	inQ := store.RelationQuery{EntityID: e.ID, Direction: store.DirectionIncoming}
	for r, err := range st.ListRelations(ctx, inQ) {
		if err != nil { // coverage-ignore: defensive: memstore.ListRelations iterator never yields a non-nil error
			break
		}
		incoming = append(incoming, r)
	}
	outQ := store.RelationQuery{EntityID: e.ID, Direction: store.DirectionOutgoing}
	if !e.Face.IsImplicit() {
		// An outgoing edge is tailed at one face; show that face's own.
		outQ.FromFace = &e.Face
	}
	for r, err := range st.ListRelations(ctx, outQ) {
		if err != nil { // coverage-ignore: defensive: memstore.ListRelations iterator never yields a non-nil error
			break
		}
		outgoing = append(outgoing, r)
	}

	mentionedBy, err := showMentionedBy(ctx, svc, e.ID)
	if err != nil {
		return err
	}
	return out.WriteEntity(withRollups(ctx, svc, []*entity.Entity{e})[0], incoming, outgoing, mentionedBy)
}

// showMentionLimit caps the backlinks `rela show` prints. The output says
// when it cut the list short.
const showMentionLimit = 50

// showMentionedBy reads the first page of id's backlinks (TKT-Q6AKUN) as the
// operator, in the default world, with each source at the face that world
// serves.
func showMentionedBy(ctx context.Context, svc *readServices, id string) (output.MentionedBy, error) {
	res, err := visibility.NewAllowAllResolver(svc.Store,
		visibility.WithFamilies(func() store.WorldScope { return svc.Families }))
	if err != nil { // coverage-ignore: defensive: NewAllowAllResolver only fails on a nil loader
		return output.MentionedBy{}, err
	}
	page, err := res.MentionedBy(ctx, visibility.WorldOf(svc.World), id, "", showMentionLimit)
	if errors.Is(err, visibility.ErrMentionsUnsupported) { // coverage-ignore: every store backend keeps the index
		return output.MentionedBy{}, nil
	}
	if err != nil {
		return output.MentionedBy{}, fmt.Errorf("read backlinks of %s: %w", id, err)
	}
	backlinks := output.MentionedBy{Truncated: page.Next != "" || page.Truncated}
	for _, h := range page.Items {
		backlinks.Items = append(backlinks.Items, output.MentionSource{
			ID: h.ID, Face: h.Face.String(), Type: h.Type,
			Title: svc.Meta.DisplayTitle(h.ID, h.Type, h.Properties),
		})
	}
	return backlinks, nil
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

// classifyReadError maps a read error onto a user-facing
// type. Today it only distinguishes ErrNotFound from every other
// error shape, but the indirection stays so future classes plug in
// without touching every caller.
func classifyReadError(id string, err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return &entityNotFoundError{ID: id}
	}
	return err
}
