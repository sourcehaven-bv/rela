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
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// DeleteCmd deletes an entity and (optionally) its relations.
type DeleteCmd struct {
	ID      string `arg:"" help:"Entity ID. On a faced type, ID@face deletes that face; the last face deletes the entity."`
	Force   bool   `short:"f" help:"Skip confirmation prompt."`
	Cascade bool   `help:"Also delete related links. An ID@face delete always removes the links tailed at that face."`
}

// Run dispatches `rela delete <id>`.
//
// The id resolves as every write does ([writeTarget]): a bare id deletes a
// faceless entity, and on a faced type is [errFaceRequired]. An `ID@face`
// deletes that face and the edges tailed at it, leaving the rest of the
// family standing (BUG-J3PBFN); the last face takes the entity.
//
// --cascade guards every delete that removes the entity: the family delete,
// and a face delete of the family's last face, which takes every incident
// edge (RR-2466U1). The edges tailed at a face that is not the last are that
// face's content, as its properties are, so such a delete always takes them.
func (c *DeleteCmd) Run(ctx context.Context, svc *writeServices) error {
	ref, err := writeTarget(ctx, svc.Store, svc.Families, svc.World, c.ID)
	if errors.Is(err, store.ErrNotFound) {
		return &entityNotFoundError{ID: c.ID}
	}
	if err != nil {
		return err
	}
	target, wholeEntity, err := deleteTarget(ctx, svc.Store, ref)
	if err != nil {
		return classifyReadError(c.ID, err)
	}

	totalRelations, err := svc.Store.CountRelations(ctx, deleteScope(ref, wholeEntity))
	if err != nil {
		return fmt.Errorf("count relations of %s: %w", c.ID, err)
	}
	if totalRelations > 0 && !c.Cascade && wholeEntity {
		return fmt.Errorf("entity %s has %d relation(s); use --cascade to delete them too", c.ID, totalRelations)
	}

	owned := 0
	if wholeEntity {
		if owned, err = countOwned(ctx, svc.Store, svc.Meta, ref.ID); err != nil {
			return fmt.Errorf("count entities owned by %s: %w", c.ID, err)
		}
	}

	if !c.Force {
		fmt.Printf("Delete %s '%s'", target.Type, svc.Meta.DisplayTitle(target.ID, target.Type, target.Properties))
		if !ref.Face.IsImplicit() {
			fmt.Printf(" at face %s", ref.Face)
		}
		if owned > 0 {
			fmt.Printf(", the %d entit(ies) it owns", owned)
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
	if ref.Face.IsImplicit() {
		result, err = svc.EntityManager.DeleteEntity(ctx, ref.ID, c.Cascade)
	} else {
		result, err = svc.EntityManager.DeleteEntityFace(ctx, ref.ID, ref.Face, c.Cascade)
	}
	if err != nil {
		if errors.Is(err, entitymanager.ErrHasRelations) {
			return fmt.Errorf("entity %s has relation(s); use --cascade to delete them too", c.ID)
		}
		return err
	}

	out.WriteSuccess("Deleted %s", c.ID)
	if n := otherEntities(result, ref.ID); n > 0 {
		out.WriteMessage("  Also deleted %d owned entit(ies)", n)
	}
	if len(result.DeletedRelations) > 0 {
		out.WriteMessage("  Also deleted %d relation(s)", len(result.DeletedRelations))
	}
	return nil
}

// deleteTarget reads the row a delete of ref names, for the confirmation
// prompt, and reports whether the delete removes the whole entity. A face
// address reads that face; it removes the entity when it is the family's
// last face. A bare id reads the family and returns its first row. It reaches
// here only when the family holds the implicit face ([writeTarget]), and the
// family delete then removes every row.
func deleteTarget(ctx context.Context, st store.Store, ref entity.Ref) (*entity.Entity, bool, error) {
	family, err := store.Family(ctx, st, ref.ID)
	if err != nil {
		return nil, false, err
	}
	if ref.Face.IsImplicit() {
		if len(family) == 0 {
			return nil, false, store.ErrNotFound
		}
		// The prompt shows one row; pick the lowest face so it is stable.
		first := family[0]
		for _, e := range family[1:] {
			if e.Face < first.Face {
				first = e
			}
		}
		return first, true, nil
	}
	for _, e := range family {
		if e.Face == ref.Face {
			return e, len(family) == 1, nil
		}
	}
	return nil, false, store.ErrNotFound
}

// deleteScope is the set of edges a delete of ref removes: every incident
// edge when the entity goes, the outgoing edges tailed at the face when only
// the face goes (see store.EntityWriter.DeleteFace).
func deleteScope(ref entity.Ref, wholeEntity bool) store.RelationQuery {
	if wholeEntity {
		return store.RelationQuery{EntityID: ref.ID, Direction: store.DirectionBoth}
	}
	face := ref.Face
	return store.RelationQuery{EntityID: ref.ID, Direction: store.DirectionOutgoing, FromFace: &face}
}

// countOwned counts the entities id owns through an `owning:` relation
// (TKT-QO14GB). Deleting id deletes them too.
func countOwned(ctx context.Context, st store.Store, meta *metamodel.Metamodel, id string) (int, error) {
	n := 0
	q := store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing}
	for r, err := range st.ListRelations(ctx, q) {
		if err != nil {
			return 0, err
		}
		if metamodel.IsOwning(meta, r.Type) {
			n++
		}
	}
	return n, nil
}

// otherEntities counts the distinct entities in result other than id: the
// owned entities a delete of id took along.
func otherEntities(result *entity.DeleteResult, id string) int {
	seen := map[string]bool{}
	for _, e := range result.DeletedEntities {
		if e.ID != id {
			seen[e.ID] = true
		}
	}
	return len(seen)
}
