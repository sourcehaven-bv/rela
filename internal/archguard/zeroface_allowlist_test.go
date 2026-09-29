package archguard

// zeroFaceAllowlist pins every zero-face read that existed when the guard was
// added (TKT-YJ17N2), per repo-relative file, as an exact count. It may only
// shrink.
//
// A zero-face read asks for the row with no face: store.Store.GetEntity(ctx,
// id), GetEntityState with a literal "" face, a two-argument getEntity (the
// shape of dataentry's former entityReader.getEntity, deleted by TKT-KQXVF7),
// or bareEntityID. A type that declares faces stores no row there
// (BUG-HC6I2T), so such a read silently finds nothing for it. DEC-NPZICR
// removes the zero face as an API; stages 1 and 2 of RES-Y6JA37 delete these
// entries.
//
// New code reads an explicit address instead; see alternatives for the
// options. Do not raise a count or add a file to make the guard pass. When you
// remove a read, lower its count here (or delete the entry); the guard fails
// on a count that is too low as well as too high, so the list stays exact. A
// count cannot tell reads apart, so removing one read and adding another in
// the same file passes; review covers that swap.
var zeroFaceAllowlist = map[string]int{
	"internal/analysis/analysis.go":         1,
	"internal/analysis/relation_order.go":   1,
	"internal/autocascade/runner.go":        1,
	"internal/cli/delete.go":                1,
	"internal/cli/trace.go":                 4,
	"internal/dataentry/gantt_handler.go":   1,
	"internal/entitymanager/apply.go":       1,
	"internal/entitymanager/cascadehost.go": 2,
	"internal/entitymanager/core.go":        4,
	"internal/importer/importer.go":         4,
	"internal/store/fsstore/entity.go":      1,
	"internal/store/fsstore/formatter.go":   1,
	"internal/store/memstore/memstore.go":   1,
	"internal/store/pgstore/entity.go":      1,
	"internal/store/sqlitestore/entity.go":  2,
	"internal/store/store.go":               1,
	"internal/tracer/tracer.go":             5,
	"internal/visibility/tracer.go":         3,
}
