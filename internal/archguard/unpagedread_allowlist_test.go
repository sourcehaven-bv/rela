package archguard

// unpagedReadAllowlist pins every unpaged whole-collection read in non-test
// code outside the store (see unpagedReads), per function, keyed
// "repo/relative/file.go#Func" or "...#Recv.Method". It may only shrink.
//
// Each entry has a category (see readCategory) and a reason that says what
// the read is for. requestPath entries are debt. The follow-ups of
// TKT-FED6FMM remove them: TKT-F2EJQY8 (dataentry), TKT-D9UO9Y4 (mcp,
// search), TKT-GIAGLQ3 (ingest, rename), TKT-P9QMEVI (lua) and TKT-I3MGCRT
// (desktopnotify).
var unpagedReadAllowlist = map[string]unpagedRead{
	// aclmap: the who-can report.
	"internal/aclmap/enumerate.go#Engine.enumeratePrincipals": {2, batchJob,
		"who-can report: enumerates every user entity and every role-conferring edge as candidate principals"},
	"internal/aclmap/mapprincipal.go#Engine.typeExceptions": {1, batchJob,
		"who-can report: walks every row of a type to find the ones whose access differs from the type baseline"},
	"internal/aclstore/principallookup.go#PrincipalLookup.LookupEntityByProperty": {1, bounded,
		"principal lookup: one equality predicate on the principal property, which the policy requires to be " +
			"unique, so at most one row"},

	// analysis: whole-graph reports.
	"internal/analysis/analysis.go#Service.FindDuplicates": {1, batchJob,
		"duplicates report: every state of every entity, through collectEntities"},
	"internal/analysis/analysis.go#Service.FindGaps": {1, batchJob,
		"id gaps report: every state of every entity, through collectEntities"},
	"internal/analysis/analysis.go#Service.FindUniqueViolations": {1, batchJob,
		"unique report: every state of every entity, through collectEntities"},
	"internal/analysis/analysis.go#Service.RunValidations": {1, batchJob,
		"validations report: every state of every entity, through collectEntities"},
	"internal/analysis/analysis.go#Service.RunValidationsFiltered": {1, batchJob,
		"scoped validations report: every state of every entity, through collectEntities, filtered after"},
	"internal/analysis/owning.go#Service.CheckOwning": {1, batchJob,
		"owning-edge check: reads every edge of each owning relation type"},
	"internal/analysis/relation_order.go#checkOrderSide": {1, batchJob,
		"relation-order check: reads every edge of an ordered relation type to find duplicate order values"},
	"internal/analysis/states.go#Service.collectStateFamilies": {1, batchJob,
		"state analysis: scans every face of every entity into families"},

	// appbuild.
	"internal/appbuild/appbuildtest/fixture.go#backfill": {1, batchJob,
		"test fixture: re-indexes every entity of the fixture store into bleve"},
	"internal/appbuild/bleveindex_shared.go#backfillBleveInto": {1, batchJob,
		"startup reindex: streams every entity into the in-memory bleve index"},
	"internal/appbuild/flowcontrol.go#FlowControl.Runs": {1, batchJob,
		"`rela flow runs`: lists every flow run row; its only caller is the CLI"},
	"internal/appbuild/idproles.go#roleHolders.usersWithProp": {1, bounded,
		"rela.role_holders script binding: one narrowing branch per role holder's subject, so it reads the " +
			"user rows holding those subjects"},
	"internal/appbuild/projectdata_sqlite.go#checkDeclaredTypes": {1, batchJob,
		"project export to sqlite: checks every entity's type before writing"},
	"internal/appbuild/projectdata_sqlite.go#copyData": {2, batchJob,
		"project export to sqlite: copies every entity and every relation"},
	"internal/appbuild/realtime_triggers.go#entitySource.Scan": {1, batchJob,
		"realtime trigger engine: one unpaged header scan of a type, handed to the engine in batches of " +
			"triggerScanPage when it (re)baselines a trigger"},
	"internal/appbuild/scheduled_mail_validate.go#Services.ValidateScheduledMailRecipients": {1, batchJob,
		"operator validation of scheduled-mail tasks: scans each task's for_each type for invalid addresses"},
	"internal/appbuild/scheduler_foreach.go#Services.ScheduledForEachEntities": {1, batchJob,
		"scheduled task for_each: selects the task's subjects from its entity type"},

	// apptest: `rela test`.
	"internal/apptest/actor.go#fileRun.list": {1, batchJob,
		"`rela test` actor:list(type): lists every readable entity of a type in a case's fresh in-memory world"},

	// cli: commands that read or rewrite every row.
	"internal/cli/analyze_prose.go#collectProse": {1, batchJob,
		"`rela analyze prose`: runs the checkers over every entity body, optionally of one type"},
	"internal/cli/export.go#ExportCmd.exportAllData":  {2, batchJob, "`rela export`: dumps every entity and relation"},
	"internal/cli/export.go#ExportCmd.exportEntities": {1, batchJob, "`rela export <type>`: dumps every entity of a type"},
	"internal/cli/fmt.go#FmtCmd.Run": {1, batchJob,
		"`rela fmt`: reformats every entity file, through formatEntities"},
	"internal/cli/fmt.go#FmtCmd.formatRelations": {1, batchJob, "`rela fmt`: reformats every relation file"},
	"internal/cli/graph.go#GraphCmd.Run":         {1, batchJob, "`rela graph`: renders every edge"},
	"internal/cli/graph.go#graphNodes":           {2, batchJob, "`rela graph`: renders every entity as a node"},
	"internal/cli/list.go#ListCmd.Run": {1, batchJob,
		"`rela list`: lists every entity of a type, through collectListEntities"},
	"internal/cli/normalize.go#NormalizeCmd.Run": {1, batchJob,
		"`rela normalize`: rewrites every entity, optionally of one type"},
	"internal/cli/renumber.go#buildRenumberPlan": {1, batchJob,
		"`rela renumber`: densifies the order values of every edge of a relation type"},

	// dataentry.
	"internal/dataentry/analyze.go#analyzeService.analyzeDuplicates": {1, batchJob,
		"/_analyze duplicates report: compares the titles of every entity"},
	"internal/dataentry/analyze.go#analyzeService.analyzeGaps": {1, batchJob,
		"/_analyze gaps report: reads every id to find holes in the numbering"},
	"internal/dataentry/analyze.go#analyzeService.analyzeProperties": {1, batchJob,
		"/_analyze properties report: validates every face row"},
	"internal/dataentry/gantt_handler.go#ganttHandler.collectGanttRound": {1, bounded,
		"gantt closure round: a GraphQuery for the descendants of the frontier, so it reads the subtree " +
			"the chart shows, not the type"},
	"internal/dataentry/helpers.go#listAllFromStore": {1, requestPath,
		"listFromStoreByTypes with no type: drains every entity in the world (unreached today, per its doc)"},
	"internal/dataentry/helpers.go#listFromStoreByTypes": {1, requestPath,
		"view and query helpers: loads every entity of each requested type"},
	"internal/dataentry/helpers.go#visibleListByTypes": {1, requestPath,
		"executeQuery without free text: loads every entity of the requested types, then gates"},
	"internal/dataentry/listpushdown.go#listPlan.run": {1, bounded,
		"list pushdown page: the plan's GraphQuery carries Limit = perPage and Offset (planListPushdown); " +
			"the matcher cannot see a field set in another function"},
	"internal/dataentry/scopedread.go#scopedHeaders": {3, requestPath,
		"the scoped collection read behind lists, kanban and views: every header of the type in the world, " +
			"directly or as the ACL-scoped GraphQuery with no Limit through collectHeaders " +
			"(the TODO(TKT-F2EJQY8) scan the budget tests name)"},

	// datamigration: migrations rewrite whole types.
	"internal/datamigration/adoptface.go#faceAdoption.plan": {1, batchJob, "face adoption: scans a type for zero-face rows to move"},
	"internal/datamigration/gc.go#scanEntities":             {1, batchJob, "migration gc: scans every entity for stale shape keys"},
	"internal/datamigration/gc.go#scanRelations":            {1, batchJob, "migration gc: scans every relation for stale shape keys"},
	"internal/datamigration/luareads.go#luaGetRelations":    {1, batchJob, "Lua migration step's get_relations: a migration reads what it rewrites"},
	"internal/datamigration/run.go#Exec.forEachEntity":      {1, batchJob, "migration steps: run over every entity of the step's type"},
	"internal/datamigration/steps.go#collectEntityFaces":    {1, batchJob, "drop-type step: collects every row of the dropped type"},
	"internal/datamigration/steps.go#collectRelations":      {1, batchJob, "relation steps: collect every edge of the step's type"},
	"internal/datamigration/steps.go#renameFaceStep.Run":    {1, batchJob, "rename-face step: scans the type for rows at the old face"},

	"internal/desktopnotify/evaluate.go#evaluation.scan": {1, requestPath,
		"desktop notification poll, for a condition queryplan.LowerScope does not lower exactly: reads every " +
			"header of the type that passes the condition's pushed equalities and judges the rest in Go. A " +
			"condition that lowers reads a capped page and counts in the store instead; TKT-I3MGCRT lowers more"},

	// docs: the documentation build's assertions over seeded projects.
	"internal/docs/assert.go#docRuntime.entityIDs":              {1, batchJob, "docs build: lists the seeded ids of a type"},
	"internal/docs/assert_read.go#seededIDs":                    {1, batchJob, "docs build: lists the seeded ids of a type for an error message"},
	"internal/docs/module.go#docRuntime.luaCount":               {1, batchJob, "docs build: counts the seeded entities of a type"},
	"internal/docs/resolvers_graph.go#docRuntime.luaResolution": {1, batchJob, "docs build: renders how a world resolves a seeded type"},

	// entitymanager: the write path.
	"internal/entitymanager/manager_order.go#assignManagedOrder": {1, bounded,
		"order on create: the assignSide closure gets orderSiblingQuery's From or To query; the matcher " +
			"does not follow closure parameters"},
	"internal/entitymanager/manager_order.go#Manager.runRenumberAfterUpdate": {3, bounded,
		"renumber after update: the renumber closure gets a From or To query of one relation type; the " +
			"matcher does not follow closure parameters"},
	"internal/entitymanager/manager_order.go#moveRelation": {1, bounded,
		"move: orderSiblingQuery sets From or To; the matcher cannot see into the helper"},
	"internal/entitymanager/manager_order.go#readableOrderSiblings": {1, bounded,
		"move: orderSiblingQuery sets From or To; the matcher cannot see into the helper"},
	"internal/entitymanager/rename.go#idTakenByOther": {1, requestPath,
		"rename dry run: scans every header for a case-folded id collision"},
	"internal/entitymanager/unique.go#checkExternalRefUnique": {1, bounded,
		"external ref uniqueness: one PropKeyEqual lookup per declaring (type, property)"},
	"internal/entitymanager/unique.go#uniqueScan.lookup": {1, bounded,
		"`unique:` check: one PropEqual GraphQuery per unique property on the value written, so it " +
			"returns the rows that already hold that value"},

	// fsimport and importer: moving a whole project.
	"internal/fsimport/copy.go#copier.copyEntities":    {1, batchJob, "fs import: copies every entity"},
	"internal/fsimport/copy.go#copier.copyRelations":   {1, batchJob, "fs import: copies every relation"},
	"internal/fsimport/source.go#source.checkLocked":   {2, batchJob, "fs import: pre-scan of every entity and relation for git-crypt stubs"},
	"internal/fsimport/verify.go#verifier.attachments": {1, batchJob, "fs import verification: every family's attachments"},
	"internal/fsimport/verify.go#verifier.entities":    {1, batchJob, "fs import verification: every entity"},
	"internal/fsimport/verify.go#verifier.relations":   {1, batchJob, "fs import verification: every relation"},
	"internal/importer/importer.go#Importer.validateRelations": {1, batchJob,
		"import: reads every header to check the imported relations' endpoints"},

	"internal/ingest/ingest.go#findTarget": {1, requestPath,
		"webhook and subscription delivery: scans the find type for the record's match values, per record"},

	// lua: script bindings. Scripts run inside requests (automations on a
	// write, actions), so a whole-type binding is request-path debt.
	"internal/lua/elevation.go#elevatedGetRelations": {1, requestPath,
		"admin.get_relations: the script's from/type/to, possibly none"},
	"internal/lua/elevation.go#elevatedListEntities": {1, requestPath,
		"admin.list_entities: every entity of a type"},
	"internal/lua/markdown.go#mdEntityRefs.luaMdEntityRefs": {1, requestPath,
		"rela.md.entity_refs: reads every entity of the named types to build a replacement map"},
	"internal/lua/runtime.go#Runtime.luaGetRelations": {1, requestPath,
		"rela.get_relations: the script's from/type/to, possibly none"},
	"internal/lua/runtime.go#Runtime.luaListEntities": {1, bounded,
		"rela.list_entities: stops the iterator at the script's limit, at most maxReadLimit (2000) rows " +
			"(TestListEntities_BoundedByDefault). Bounded on the ACL pushdown path, which every server " +
			"wiring takes, and on pgstore and sqlitestore, which page by keyset; ScriptReader's " +
			"load-then-filter fallback and memstore still read the whole type before the break"},

	"internal/mailtemplate/mailtemplate.go#buildSection": {1, batchJob,
		"scheduled mail render: a section lists the entities of its declared type"},

	// mcp.
	"internal/mcp/tools_analysis.go#Server.handleAnalyzeProperties": {1, batchJob,
		"analyze properties tool: validates every face row"},
	"internal/mcp/tools_analysis.go#Server.handleAnalyzeUnique": {1, batchJob,
		"analyze unique tool: compares the unique values of every entity of each type"},
	"internal/mcp/tools_entity.go#Server.handleListEntities": {1, requestPath,
		"list_entities tool: reads the whole type (or every type), then filters and applies the limit in Go"},

	// remotesync: a sync pass over the mapped types.
	"internal/remotesync/push.go#pusher.localRows": {1, batchJob, "remote sync push: every local row of a mapped type"},
	"internal/remotesync/relsync.go#relSync.push":  {1, batchJob, "remote sync push: every local edge of a mapped relation type"},
	"internal/remotesync/run.go#runner.findByRef": {1, bounded,
		"remote sync: a PropKeyEqual lookup on the external ref, at most one row per declaring type"},

	// schema: analysis over the whole graph.
	"internal/schema/cardinality.go#subjectCache.list": {1, batchJob,
		"cardinality analysis: every face of each subject type"},
	"internal/schema/cardinality.go#countEdges": {1, batchJob,
		"cardinality analysis: every edge of a relation type, narrowed to the scope's ids when there is one"},
	"internal/schema/validate_properties.go#ValidateEntityProperties": {1, batchJob,
		"property validation report: every entity"},
	"internal/schema/validate_properties.go#ValidateRelationProperties": {1, batchJob,
		"property validation report: every relation"},

	// search.
	"internal/search/index.go#Service.listAll": {1, requestPath,
		"search with no text: reads every entity in the world, then filters by type and properties"},
	"internal/search/index.go#Service.scanFaces": {1, requestPath,
		"search with an admit gate and no text: reads every face of every entity, then filters by type"},

	// tracer.
	"internal/tracer/tracer.go#GenericTracer.FindOrphans": {2, batchJob,
		"orphan report: every header and every edge"},

	"internal/validator/validator.go#GenericValidator.loadCandidates": {1, batchJob,
		"validation report (analyze validations): every entity of each rule's target types"},

	// visibility: the read-side ACL readers.
	"internal/visibility/externalref.go#findByExternalRef": {1, bounded,
		"external ref lookup: one PropKeyEqual GraphQuery per holder type"},
	"internal/visibility/luareader.go#ScriptReader.listGatedThenRanked": {1, passThrough,
		"ScriptReader.ListEntities' load-then-filter fallback: runs its caller's query at every face; " +
			"the HolderLookup callers pass a whole type"},
	"internal/visibility/pushdown.go#listPushdown": {2, passThrough,
		"ScriptReader.ListEntities' pushdown: runs its caller's query with the ACL scope added"},
	"internal/visibility/tracer.go#VisibleTracer.FindOrphans": {2, batchJob,
		"gated orphan report: every header (through Resolver.storedHeaders) and every edge"},

	"internal/worldreader/relations.go#RelationReader.Neighbors": {1, bounded,
		"one entity's edges: setEndpoint sets From, To or EntityID through a pointer the matcher cannot follow"},
}
