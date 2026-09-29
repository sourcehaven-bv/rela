package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// DeleteCmd deletes an entity and (optionally) its relations.
type DeleteCmd struct {
	ID      string `arg:"" help:"Entity ID, or ID@face to delete one face."`
	Force   bool   `short:"f" help:"Skip confirmation prompt."`
	Cascade bool   `help:"Also delete related links. An ID@face delete always removes the links tailed at that face."`
}

// Run dispatches `rela delete <id>`.
//
// A bare id deletes the whole family: every face and every incident edge. An
// `ID@face` deletes that face and the edges tailed at it, leaving the rest of
// the family standing (BUG-J3PBFN). Either way the row is found by its
// address, so a faced entity is not reported missing.
//
// --cascade guards the family delete only. The edges tailed at a face are
// that face's content, as its properties are, so a face delete always takes
// them; the data-entry app and the Lua binding do the same.
func (c *DeleteCmd) Run(ctx context.Context, svc *writeServices) error {
	ref, err := entity.ParseRef(c.ID)
	if err != nil {
		return &entityNotFoundError{ID: c.ID}
	}
	target, err := deleteTarget(ctx, svc.Store, ref)
	if err != nil {
		return classifyReadError(c.ID, err)
	}

	totalRelations, err := svc.Store.CountRelations(ctx, deleteScope(ref))
	if err != nil {
		return fmt.Errorf("count relations of %s: %w", c.ID, err)
	}
	if totalRelations > 0 && !c.Cascade && ref.Face.IsDefault() {
		return fmt.Errorf("entity %s has %d relation(s); use --cascade to delete them too", c.ID, totalRelations)
	}

	if !c.Force {
		fmt.Printf("Delete %s '%s'", target.Type, svc.Meta.DisplayTitle(target.ID, target.Type, target.Properties))
		if !ref.Face.IsDefault() {
			fmt.Printf(" at face %s", ref.Face)
		}
		if totalRelations > 0 {
			fmt.Printf(" and %d relation(s)", totalRelations)
		}
		fmt.Print("? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		response, readErr := reader.ReadString('\n')
		if readErr != nil {
			return fmt.Errorf("failed to read input: %w", readErr)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			out.WriteMessage("Cancelled")
			return nil
		}
	}

	var result *entity.DeleteResult
	if ref.Face.IsDefault() {
		result, err = svc.EntityManager.DeleteEntity(ctx, ref.ID, c.Cascade)
	} else {
		result, err = svc.EntityManager.DeleteEntityFace(ctx, ref.ID, ref.Face)
	}
	if err != nil {
		if errors.Is(err, entitymanager.ErrHasRelations) {
			return fmt.Errorf("entity %s has relation(s); use --cascade to delete them too", c.ID)
		}
		return err
	}

	out.WriteSuccess("Deleted %s", c.ID)
	if len(result.DeletedRelations) > 0 {
		out.WriteMessage("  Also deleted %d relation(s)", len(result.DeletedRelations))
	}
	return nil
}

// deleteTarget reads the row a delete of ref names, for the confirmation
// prompt. A face address reads that face. A bare id reads the family and
// returns its first row: a faced type stores no bare row, and the family
// delete removes every face anyway.
func deleteTarget(ctx context.Context, st store.Store, ref entity.Ref) (*entity.Entity, error) {
	if !ref.Face.IsDefault() {
		return st.GetEntityState(ctx, ref.ID, ref.Face)
	}
	q := store.EntityQuery{IDs: []string{ref.ID}, Faces: store.AllFaces()}
	for e, err := range st.ListEntities(ctx, q) {
		if err != nil {
			return nil, err
		}
		if e.ID == ref.ID {
			return e, nil
		}
	}
	return nil, store.ErrNotFound
}

// deleteScope is the set of edges a delete of ref removes: every incident
// edge for a family, the outgoing edges tailed at the face for a face.
func deleteScope(ref entity.Ref) store.RelationQuery {
	if ref.Face.IsDefault() {
		return store.RelationQuery{EntityID: ref.ID, Direction: store.DirectionBoth}
	}
	face := ref.Face
	return store.RelationQuery{EntityID: ref.ID, Direction: store.DirectionOutgoing, FromFace: &face}
}
