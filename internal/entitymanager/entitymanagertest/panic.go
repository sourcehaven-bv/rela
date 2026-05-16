// Package entitymanagertest provides test doubles for the
// entitymanager.EntityManager interface.
//
// PanicOnUse satisfies the interface but panics from every method. Wire it
// into tests that need to construct a writer runtime (which requires a
// non-nil EntityManager) but exercise only read paths — an accidental
// mutation will fail loudly instead of silently no-oping.
package entitymanagertest

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
)

// PanicOnUse is an entitymanager.EntityManager whose every method panics.
// Use it when a test's code path should never reach a mutation.
type PanicOnUse struct{}

var _ entitymanager.EntityManager = PanicOnUse{}

func (PanicOnUse) CreateEntity(context.Context, *entity.Entity,
	entity.CreateOptions) (*entity.CreateResult, error) {
	panic("entitymanagertest.PanicOnUse.CreateEntity: not expected in this test")
}

func (PanicOnUse) UpdateEntity(context.Context,
	*entity.Entity) (*entity.UpdateResult, error) {
	panic("entitymanagertest.PanicOnUse.UpdateEntity: not expected in this test")
}

func (PanicOnUse) DeleteEntity(context.Context, string,
	bool) (*entity.DeleteResult, error) {
	panic("entitymanagertest.PanicOnUse.DeleteEntity: not expected in this test")
}

func (PanicOnUse) RenameEntity(context.Context, string, string,
	entity.RenameOptions) (*entity.RenameResult, error) {
	panic("entitymanagertest.PanicOnUse.RenameEntity: not expected in this test")
}

func (PanicOnUse) CreateRelation(context.Context, string, string, string,
	entity.RelationOptions) (*entity.Relation, error) {
	panic("entitymanagertest.PanicOnUse.CreateRelation: not expected in this test")
}

func (PanicOnUse) UpdateRelation(context.Context, string, string, string,
	entity.RelationOptions) (*entity.Relation, error) {
	panic("entitymanagertest.PanicOnUse.UpdateRelation: not expected in this test")
}

func (PanicOnUse) DeleteRelation(context.Context, string, string, string) error {
	panic("entitymanagertest.PanicOnUse.DeleteRelation: not expected in this test")
}
