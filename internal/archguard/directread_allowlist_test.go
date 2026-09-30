package archguard

// directReadAllowlist pins the direct entity reads on the gated surfaces
// (see directReads). It may only shrink: PR 3 and PR 5 of TKT-2528AB remove
// entries until only write-prep reads remain.
//
// Most dataentry entries are id-batch reads for a collection page
// (TKT-1U8XYN): they load the rows or headers first and gate the batch
// afterwards, because a resolver call per row is the per-row lookup that
// ticket removed. They leave the list when the resolver gains a batch form.
var directReadAllowlist = map[string]allowed{
	"internal/dataentry/api_v1.go": {1, "relation title filter: id-batch header load for a page's " +
		"neighbors, face-gated and row-gated afterwards"},
	"internal/dataentry/document.go": {1, "loadEntry reads the entry row the document route already " +
		"gated; the render plumbing is shared with export, which moves in PR 4"},
	"internal/dataentry/entityreader.go": {1, "readWritePrep: write-prep, liveness and relation-source policy read, never served"},
	"internal/dataentry/gantt_handler.go": {1, "gantt root read: raw so the roll-up can run " +
		"gate-then-redact-then-fold; the type verdict and visibility.Redact run before any value is served"},
	"internal/dataentry/queryservice.go": {1, "loadHitHeaders: id-batch header load for search hits, " +
		"gated afterwards"},
	"internal/dataentry/rowcontent.go": {1, "loadRows: id-batch raw row load; every caller gates or redacts before serving"},
	"internal/dataentry/views.go": {3, "view collections: id-batch body and header loads, " +
		"gated afterwards"},
	"internal/dataentry/views_handler.go": {2, "visibleTitles: id-batch header load for relation " +
		"columns, gated by the viewReader"},
	"internal/dataentry/viewworld.go": {1, "view traversal: id-batch header load in the view's world, " +
		"gated afterwards"},
	"internal/dataentry/visiblereader.go": {1, "storedFacesOf: type and face lookup that decides " +
		"liveness and the gate's type, never what is served"},
	"internal/dataentry/worldneighbors.go": {1, "resolveHeads: id-batch head resolution in the " +
		"request's world, gated afterwards"},
	"internal/mcp/tools_entity.go": {1, "hydrateHits: id-batch body load for search hits " +
		"through the gated GraphReader, keyed by the hit's face"},
}
