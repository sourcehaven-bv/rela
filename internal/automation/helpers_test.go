package automation

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// buildEntity returns the built entity from a testutil EntityBuilder.
func buildEntity(b *testutil.EntityBuilder) *entity.Entity {
	return b.Build()
}

// buildRelation returns the built relation from a testutil RelationBuilder.
func buildRelation(b *testutil.RelationBuilder) *entity.Relation {
	return b.Build()
}
