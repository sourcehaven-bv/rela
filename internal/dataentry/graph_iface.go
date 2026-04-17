package dataentry

import "github.com/Sourcehaven-BV/rela/internal/model"

// EntityGraph defines the read-only graph operations used by the data entry app.
// Satisfied by *graph.Graph today; will be replaced by a store.Store adapter later.
type EntityGraph interface {
	GetNode(id string) (*model.Entity, bool)
	NodesByType(entityType string) []*model.Entity
	AllNodes() []*model.Entity
	AllIDs() []string
	AllEdges() []*model.Relation
	OutgoingEdges(id string) []*model.Relation
	IncomingEdges(id string) []*model.Relation
	FindOrphans() []*model.Entity
}
