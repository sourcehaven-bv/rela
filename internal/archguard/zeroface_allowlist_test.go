package archguard

// zeroFaceAllowlist pins every zero-face read that existed when the guard was
// added (TKT-YJ17N2), per repo-relative file, as an exact count. It may only
// shrink.
//
// A zero-face read asks for the row with no face: store.Store.GetEntity(ctx,
// id), GetEntityState with a literal "" face, dataentry's
// entityReader.getEntity, or bareEntityID. A type that declares faces stores
// no row there (BUG-HC6I2T), so such a read silently finds nothing for it.
// DEC-NPZICR removes the zero face as an API; stages 1 and 2 of RES-Y6JA37
// delete these entries.
//
// New code reads an explicit address instead: store.GetEntityState with a
// real face, store.GetEntityAt, dataentry's visibleReader.getVisibleRef and
// entityReader.getEntityRef, or the visibility readers (ScriptReader,
// UnrestrictedReader). Do not raise a count or add a file to make the guard
// pass. When you remove a read, lower its count here (or delete the entry);
// the guard fails on a count that is too low as well as too high, so the list
// stays exact. A count cannot tell reads apart, so removing one read and
// adding another in the same file passes; review covers that swap.
var zeroFaceAllowlist = map[string]int{
	"internal/aclmap/can.go":                      1,
	"internal/aclmap/canrelation.go":              1,
	"internal/aclmap/whocan.go":                   1,
	"internal/analysis/analysis.go":               1,
	"internal/analysis/relation_order.go":         1,
	"internal/appbuild/appbuild.go":               1,
	"internal/appbuild/scheduled_mail.go":         1,
	"internal/appbuild/scheduler_foreach.go":      1,
	"internal/attachment/attachment.go":           2,
	"internal/autocascade/runner.go":              1,
	"internal/cli/acl_can.go":                     1,
	"internal/cli/acl_can_relation.go":            1,
	"internal/cli/create.go":                      1,
	"internal/cli/delete.go":                      1,
	"internal/cli/export.go":                      2,
	"internal/cli/render.go":                      1,
	"internal/cli/restore.go":                     1,
	"internal/cli/sync/force.go":                  1,
	"internal/cli/sync/push.go":                   2,
	"internal/cli/sync/splice.go":                 1,
	"internal/cli/trace.go":                       4,
	"internal/dataentry/actions.go":               1,
	"internal/dataentry/affordances.go":           2,
	"internal/dataentry/analyze.go":               2,
	"internal/dataentry/api_v1.go":                2,
	"internal/dataentry/app.go":                   2,
	"internal/dataentry/caldav_write.go":          2,
	"internal/dataentry/entityreader.go":          1,
	"internal/dataentry/export.go":                1,
	"internal/dataentry/export_list.go":           1,
	"internal/dataentry/gantt_handler.go":         1,
	"internal/dataentry/handlers_attachment.go":   2,
	"internal/dataentry/history_restore.go":       1,
	"internal/dataentry/mentions.go":              1,
	"internal/dataentry/sync.go":                  1,
	"internal/dataentry/webhook_routes.go":        1,
	"internal/dataentry/worldneighbors.go":        2,
	"internal/datamigration/steps.go":             1,
	"internal/docs/resolvers_graph.go":            1,
	"internal/docs/seed.go":                       1,
	"internal/entitymanager/apply.go":             1,
	"internal/entitymanager/cascadehost.go":       2,
	"internal/entitymanager/core.go":              4,
	"internal/importer/importer.go":               4,
	"internal/lua/elevation.go":                   1,
	"internal/lua/runtime.go":                     3,
	"internal/store/fsstore/entity.go":            1,
	"internal/store/fsstore/formatter.go":         1,
	"internal/store/memstore/memstore.go":         1,
	"internal/store/pgstore/entity.go":            1,
	"internal/store/sqlitestore/entity.go":        2,
	"internal/store/store.go":                     1,
	"internal/tracer/tracer.go":                   5,
	"internal/validationgraph/validationgraph.go": 1,
	"internal/visibility/tracer.go":               3,
}
