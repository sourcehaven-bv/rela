package sync

// orderForApply sorts a set of changes into a safe application order, the same
// rule for push and pull (RR-YHGJHG). There is no batch transaction; ordering is
// what keeps each individual idempotent write from referencing an endpoint that
// does not exist yet (on create/update) or that was already removed (on delete):
//
//	upserts (create/update): entities BEFORE relations — a relation's endpoints
//	    must exist before the relation is written.
//	deletes:                 relations BEFORE entities — remove a relation before
//	    the entity it points at, so no relation is ever orphaned mid-batch.
//
// Within each of the four buckets, order is stable by key for deterministic
// reports and reproducible tests. Because convergence is by idempotent replay, a
// wrong-but-recoverable order would still converge on a re-run; this ordering
// makes the common case succeed on the first pass.
func orderForApply[T any](items []T, classify func(T) (kind Kind, deleted bool), key func(T) string) []T {
	var (
		upsertEntities  []T
		upsertRelations []T
		deleteRelations []T
		deleteEntities  []T
	)
	for _, it := range items {
		kind, deleted := classify(it)
		switch {
		case !deleted && kind == KindEntity:
			upsertEntities = append(upsertEntities, it)
		case !deleted && kind == KindRelation:
			upsertRelations = append(upsertRelations, it)
		case deleted && kind == KindRelation:
			deleteRelations = append(deleteRelations, it)
		default: // deleted && KindEntity
			deleteEntities = append(deleteEntities, it)
		}
	}
	stableByKey(upsertEntities, key)
	stableByKey(upsertRelations, key)
	stableByKey(deleteRelations, key)
	stableByKey(deleteEntities, key)

	out := make([]T, 0, len(items))
	out = append(out, upsertEntities...)
	out = append(out, upsertRelations...)
	out = append(out, deleteRelations...)
	out = append(out, deleteEntities...)
	return out
}

// stableByKey sorts items in place by their string key (insertion sort keeps it
// dependency-free and the slices are small — one project's worth of changes).
func stableByKey[T any](items []T, key func(T) string) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && key(items[j]) < key(items[j-1]); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}
