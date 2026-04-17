package workspace

import (
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/repository"
)

// Tx is a unit of mutation against a Workspace. It wraps an open
// repository.Tx and accumulates graph mutations that are applied to the
// in-memory graph only after the repository transaction successfully
// commits.
//
// On rollback (the WithTx callback returning an error), the staged
// disk writes are removed by repository.Transaction and the graph is
// not touched. The workspace is byte-identical to its pre-transaction
// state on the read paths.
//
// Read methods (Meta, Graph, GetEntity, ...) read from the workspace's
// current committed state at the moment of the call. They do NOT see
// the transaction's own pending writes (no read-your-own-writes), and
// they do NOT take a snapshot frozen at WithTx entry — they delegate
// to the live graph, which is safe because WithTx holds reloadMu and
// excludes concurrent reloads. Direct-write paths (CreateEntity,
// UpdateEntity, ...) on the same workspace are NOT excluded by reloadMu
// today; callers must serialize them externally (e.g. via App.writeMu
// in the data-entry server).
//
// A Tx must not outlive the WithTx callback that created it. Methods
// called after the callback returns will operate on a closed repository
// transaction and return errors or panic.
type Tx struct {
	ws     *Workspace
	repoTx repository.Tx
	base   *workspaceState

	// Pending graph mutations. Applied to the live graph only after the
	// repository transaction commits successfully. On rollback, these
	// are discarded — the graph is never touched.
	addNodes    []*model.Entity
	removeNodes []string
	addEdges    []*model.Relation
	removeEdges []removedEdge
}

// removedEdge is the three-tuple needed to remove an edge from the graph.
type removedEdge struct {
	from    string
	relType string
	to      string
}

// --- Read methods ---

// Meta returns the metamodel snapshot captured at transaction start.
// Subsequent reloads of the workspace metamodel are NOT visible to this
// transaction.
func (tx *Tx) Meta() *metamodel.Metamodel { return tx.base.meta }

// Graph returns the graph snapshot captured at transaction start.
// The graph is the workspace's live graph; reads see committed state but
// not the pending mutations staged on this Tx.
func (tx *Tx) Graph() *graph.Graph { return tx.base.graph }

// GetEntity returns an entity from the base graph snapshot.
func (tx *Tx) GetEntity(id string) (*model.Entity, bool) {
	return tx.base.graph.GetNode(id)
}

// GetRelation returns a relation from the base graph snapshot.
func (tx *Tx) GetRelation(from, relType, to string) (*model.Relation, bool) {
	return tx.base.graph.GetEdge(from, relType, to)
}

// IncomingEdges returns incoming relations for an entity from the base
// graph snapshot.
func (tx *Tx) IncomingEdges(id string) []*model.Relation {
	return tx.base.graph.IncomingEdges(id)
}

// OutgoingEdges returns outgoing relations for an entity from the base
// graph snapshot.
func (tx *Tx) OutgoingEdges(id string) []*model.Relation {
	return tx.base.graph.OutgoingEdges(id)
}

// --- Write methods ---

// WriteEntity stages an entity write. The on-disk file is written to a
// temporary location and renamed atomically when the transaction commits.
// The graph is updated only after the rename succeeds.
func (tx *Tx) WriteEntity(entity *model.Entity) error {
	if err := tx.repoTx.WriteEntity(entity, tx.base.meta); err != nil {
		return err
	}
	tx.addNodes = append(tx.addNodes, entity)
	return nil
}

// WriteRelation stages a relation write. Same atomicity guarantees as
// WriteEntity.
func (tx *Tx) WriteRelation(rel *model.Relation) error {
	if err := tx.repoTx.WriteRelation(rel); err != nil {
		return err
	}
	tx.addEdges = append(tx.addEdges, rel)
	return nil
}

// DeleteEntity stages an entity deletion. The on-disk file is removed at
// commit time; the graph node is removed only after the disk delete
// succeeds.
func (tx *Tx) DeleteEntity(entityType, id string) error {
	if err := tx.repoTx.DeleteEntity(entityType, id, tx.base.meta); err != nil {
		return err
	}
	tx.removeNodes = append(tx.removeNodes, id)
	return nil
}

// DeleteRelation stages a relation deletion.
func (tx *Tx) DeleteRelation(from, relType, to string) error {
	if err := tx.repoTx.DeleteRelation(from, relType, to); err != nil {
		return err
	}
	tx.removeEdges = append(tx.removeEdges, removedEdge{from: from, relType: relType, to: to})
	return nil
}

// applyGraphMutations applies all pending graph mutations to the live
// graph and updates the search index to match. Called by
// Workspace.WithTx after the repository transaction has committed
// successfully.
//
// Order: removes first, then adds. Within removes, edges before nodes
// (so edges aren't dangling against missing nodes); within adds, nodes
// before edges (so edges find both endpoints).
//
// Note that graph.RemoveNode also removes any incident edges as a side
// effect, so an explicit removeEdges entry that targets a node about
// to be removed is harmless. Operations are best-effort: a removeEdges
// or removeNodes entry that doesn't match anything in the live graph
// is silently dropped (the disk transaction already committed, so we
// can't bail out).
func (tx *Tx) applyGraphMutations() {
	g := tx.base.graph
	st := tx.base.store

	// Remove edges first, then nodes.
	for _, e := range tx.removeEdges {
		g.RemoveEdge(e.from, e.relType, e.to)
		mirrorRelationDelete(st, e.from, e.relType, e.to)
	}
	for _, id := range tx.removeNodes {
		g.RemoveNode(id)
		mirrorEntityDelete(st, id)
		// Keep the search index in sync with the graph.
		tx.ws.removeFromIndex(id)
	}

	// Add nodes, then edges.
	for _, n := range tx.addNodes {
		g.AddNode(n)
		mirrorEntityUpsert(st, n)
		// Keep the search index in sync with the graph.
		tx.ws.indexEntity(n)
	}
	for _, r := range tx.addEdges {
		g.AddEdge(r)
		mirrorRelationCreate(st, r)
	}
}
