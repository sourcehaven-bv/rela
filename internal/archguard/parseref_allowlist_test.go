package archguard

// parseRefAllowlist pins every entity.ParseRef call in the edge trees (see
// parseRefCalls), per repo-relative file. It may only shrink.
//
// Each site parses a caller's address as a Ref, so a bare id becomes the
// implicit face. For a faceless type that is the right row; for a faced type
// it is a row that does not exist. The reads move to entity.ParseAddress when
// every read route binds a world (TKT-7IZHP0 PR 5b), the writes when bare-id
// writes resolve through the resolver (PR 7). Each entry names which.
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
	"internal/mcp/tools_relation.go": {2, "relation tools: the FROM argument names the tail of the edge " +
		"key, a serialized coordinate. Write: PR 7"},
	"internal/mcp/tools_helpers.go":    {1, "readable, the pre-write existence gate on an address. Write: PR 7"},
	"internal/mcp/tools_attachment.go": {1, "the not-found hint asks whether a face was named. Write: PR 7"},
	"internal/mcp/resources.go": {1, "relation resource FROM segment names the edge tail, a serialized " +
		"coordinate. Read: PR 5b"},
	"internal/mcp/tools_entity.go": {1, "delete_entity: bare id deletes the family, ID@face one face. " +
		"Write: PR 7"},

	// lua
	"internal/lua/runtime.go": {2, "writeTargetReadable and deleteByAddress: the write gate and the " +
		"family-or-face delete. Write: PR 7"},

	// cli
	"internal/cli/address.go": {2, "readAddress and requireAddressExists, the CLI read helpers that " +
		"resolve a bare id in a world. Read: PR 5a"},
	"internal/cli/acl_can.go": {1, "rejects a face address for the no-policy path; asks only whether a " +
		"face was named. Read: PR 5a"},
	"internal/cli/delete.go":          {1, "delete: bare id deletes the family, ID@face one face. Write: PR 7"},
	"internal/cli/attach.go":          {1, "attach target address. Write: PR 7"},
	"internal/cli/detach.go":          {1, "detach target address. Write: PR 7"},
	"internal/cli/history_address.go": {1, "history address; history is per face row. Read: PR 5a"},
	"internal/cli/unlink.go": {1, "unlink FROM names the tail of the edge key, a serialized " +
		"coordinate. Write: PR 7"},
	"internal/cli/attachments.go": {1, "attachments listing address. Read: PR 5a"},
}
