package cli

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// UnlinkCmd removes a relation between entities.
type UnlinkCmd struct {
	From     string `arg:"" help:"Source entity ID, or ID@face for an edge tailed at a face."`
	Relation string `arg:"" help:"Relation type."`
	To       string `arg:"" help:"Target entity ID."`
}

// Run dispatches `rela unlink <from> <relation> <to>`.
//
// The tail is part of a relation's identity, so `ID@face` names the
// content-scoped edge on that face and a bare id the default-tail edge
// (BUG-J3PBFN).
func (c *UnlinkCmd) Run(ctx context.Context, svc *writeServices) error {
	from, err := entity.ParseRef(c.From)
	if err != nil {
		return fmt.Errorf("relation not found: %s --%s--> %s", c.From, c.Relation, c.To)
	}
	exists, err := relationExists(ctx, svc.Store, from, c.Relation, c.To)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("relation not found: %s --%s--> %s", c.From, c.Relation, c.To)
	}
	key := entity.RelationKey{From: from.ID, FromFace: from.Face, Type: c.Relation, To: c.To}
	if err := svc.EntityManager.DeleteRelation(ctx, key); err != nil {
		return err
	}
	out.WriteSuccess("Removed link: %s --%s--> %s", c.From, c.Relation, c.To)
	return nil
}

// relationExists reports whether the edge tailed at from exists.
func relationExists(ctx context.Context, st store.Store, from entity.Ref, relType, to string) (bool, error) {
	face := from.Face
	q := store.RelationQuery{From: from.ID, FromFace: &face, Type: relType, To: to}
	n, err := st.CountRelations(ctx, q)
	if err != nil {
		return false, fmt.Errorf("look up relation: %w", err)
	}
	return n > 0, nil
}
