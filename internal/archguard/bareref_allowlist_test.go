package archguard

// bareRefAllowlist pins every bare entity.Ref literal in non-test code (see
// bareRefs), per repo-relative file. It may only shrink.
//
// A bare Ref is one of two things. As a store address it names the zero-face
// row, which a faceless type has and a faced type does not (DEC-NPZICR). As a
// key for visibility.Resolver.ResolveHeaders it is a world address: the
// resolver picks the face the request's world selects. Each entry says which
// of the two it is and why it is right.
var bareRefAllowlist = map[string]allowed{
	"internal/dataentry/rowcontent.go": {1, "loadDefaultFaceRows: the default world's rows for " +
		"entityReader, whose routes serve only the default world (TKT-7IZHP0)"},
	"internal/dataentry/gantt_handler.go": {1, "gantt root read: every source type is faceless " +
		"(ganttHasFacedSource returns early otherwise), so the zero face is the root's only row"},
	"internal/mcp/convert.go": {1, "outgoing edge head passed to ResolveHeaders: a world address, " +
		"since a relation head names the entity, not a face"},
	"internal/mcp/tools_analysis.go": {2, "orphan ids passed to ResolveHeaders and used as its " +
		"result key: world addresses for the family's title"},
}
