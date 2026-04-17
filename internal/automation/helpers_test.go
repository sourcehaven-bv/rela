package automation

import (
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// buildEntity converts a testutil EntityBuilder output to *entity.Entity.
// Bridges testutil (which still returns *model.Entity) to automation's
// *entity.Entity API.
func buildEntity(b *testutil.EntityBuilder) *entity.Entity {
	return model.EntityToDomain(b.Build())
}

// buildRelation converts a testutil RelationBuilder output to *entity.Relation.
func buildRelation(b *testutil.RelationBuilder) *entity.Relation {
	return model.RelationToDomain(b.Build())
}
