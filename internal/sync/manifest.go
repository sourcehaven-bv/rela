// Package sync holds backend-neutral value types shared by the sync feature
// (FEAT-NJ9FEN). It deliberately depends on nothing storage-specific so that
// both the producer (pgstore, postgres build only) and the consumer (the
// data-entry sync HTTP handler) can reference these types without coupling to
// each other or pulling pgx into the default build.
package sync

// ManifestEntry is one change in a sync manifest: a record created, updated, or
// deleted since a cursor. The producer (pgstore.ManifestSince) emits a slice of
// these; the sync server serializes them onto the wire for the client to diff
// against its index.
//
// Kind is "e" (entity) or "r" (relation). For an entity, IDA is the id and
// IDB/IDC are empty; Typ is the entity type. For a relation, IDA/IDB/IDC are
// from_id/rel_type/to_id and Typ is empty. Deleted is true when the entry is a
// tombstone (the live row no longer exists). Seq is the change's position in the
// global order — the highest Seq in a manifest is the next cursor.
type ManifestEntry struct {
	Kind    string
	IDA     string
	IDB     string
	IDC     string
	Typ     string
	Deleted bool
	Seq     int64
}
