package archguard

// parseRefAllowlist pins every entity.ParseRef call in the edge trees (see
// parseRefCalls), per repo-relative file. It may only shrink.
//
// Each site parses a caller's address as a Ref, so a bare id becomes the
// implicit face. For a faceless type that is the right row; for a faced type
// it is a row that does not exist. Face-level writes resolve a bare id
// through visibility.Resolver.WriteTarget instead. What is left either keys a
// stored coordinate (an edge tail, a lineage, a family-or-face delete) and
// stays, or is a read that moves to entity.ParseAddress (TKT-7IZHP0 PR 5b).
// Each entry names which.
var parseRefAllowlist = map[string]allowed{
	// dataentry
	"internal/dataentry/comments_handler.go": {1, "comment route id segment; the thread keys on the " +
		"parsed Ref. Read: PR 5b"},
	"internal/dataentry/history_handler.go": {1, "history route id segment; history is per face row. " +
		"Read: PR 5b"},
	"internal/dataentry/relation_history_handler.go": {1, "relation history FROM segment names the tail " +
		"of a stored relation key, a serialized coordinate; stays or moves with PR 2's tail grammar"},
	"internal/dataentry/views_handler.go": {1, "view route id segment: bare id for the row gate, face " +
		"for the engine. Read: PR 5b"},
	"internal/dataentry/entityref.go": {1, "isExplicitAddress only asks whether a face is named, which " +
		"is Address.Named. Read: PR 5b"},
	"internal/dataentry/relation_read_handler.go": {1, "relation read FROM address; a bare id takes the " +
		"identity-tail path. Read: PR 5b"},
	"internal/dataentry/views.go":         {2, "executeView and executeViewWhole parse the entry id. Read: PR 5b"},
	"internal/dataentry/visiblereader.go": {1, "untypedAddress, the untyped read by address. Read: PR 5b"},
	"internal/dataentry/export_document.go": {1, "document export id segment, also the reserved-segment " +
		"refusal. Read: PR 5b"},

	// mcp
	"internal/mcp/tools_relation.go": {1, "relationTail: an identity-scoped edge hangs on the entity, so " +
		"its FROM is parsed, not resolved to a face. Stays"},
	"internal/mcp/tools_helpers.go":    {1, "readable, the pre-write existence gate on an address. Stays"},
	"internal/mcp/tools_attachment.go": {1, "the read not-found hint asks whether a face was named. Stays"},
	"internal/mcp/resources.go": {1, "relation resource FROM segment names the edge tail, a serialized " +
		"coordinate. Read: PR 5b"},
	"internal/mcp/tools_entity.go": {1, "delete_entity: bare id deletes the family, ID@face one face. " +
		"Stays"},

	// lua
	"internal/lua/runtime.go": {2, "writeTargetReadable and deleteByAddress: the write gate and the " +
		"family-or-face delete. Stays"},

	// cli
	"internal/cli/delete.go": {1, "delete: bare id deletes the family, ID@face one face. Stays"},
	"internal/cli/history_address.go": {1, "history, restore and purge address a stored lineage, keyed " +
		"by (id, face), not a world read: a bare id is the zero-face lineage, and a faced id is refused " +
		"with its faces named (BUG-4SYAA6). Choosing a face by world would purge or restore a face the " +
		"operator did not name. Stays"},

	// aclmap
	"internal/aclmap/target.go": {1, "the operator's report address; the parsed Ref is also the tail of " +
		"the relation subject can-relation builds, a serialized coordinate. A bare id is the entity as a " +
		"whole, checked against every stored face. Moves with PR 6"},

	// importer
	"internal/importer/importer.go": {5, "import files carry serialized stored keys, not user addresses: " +
		"a bare id in a file is the implicit face, the row it was exported from (design G21)"},
}
