package docs

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// SeedOp is one recorded seed mutation (a create or a link). The manual's
// create()/link() islands record these; they are applied to the in-memory graph
// immediately (for the Tier-A resolvers entity{}/count{}/graph{}) and REPLAYED
// verbatim against a fresh fsstore-backed temp project when a screenshot{} island
// needs the SPA to render them. One recorder → both stores → they cannot diverge
// (DR-S2).
type SeedOp struct {
	// Kind is "create" or "link".
	Kind string
	// create fields:
	Type       string
	ID         string
	Properties map[string]any
	Content    string
	// link fields:
	From    string
	RelType string
	To      string
}

// ApplySeed replays recorded seed ops against a store, using RAW store writes
// (no entitymanager) so automations can't mutate the fixture. Used for both the
// in-memory store (phase-2 resolvers) and the screenshot temp project's store
// (Tier B), so the two representations cannot diverge (DR-S2).
func ApplySeed(ctx context.Context, st store.Store, ops []SeedOp) error {
	for _, op := range ops {
		switch op.Kind {
		case "create":
			e := &entity.Entity{ID: op.ID, Type: op.Type, Properties: op.Properties, Content: op.Content}
			if err := st.CreateEntity(ctx, e); err != nil {
				return err
			}
		case "link":
			if _, err := st.CreateRelation(ctx, op.From, op.RelType, op.To, nil); err != nil {
				return err
			}
		}
	}
	return nil
}
