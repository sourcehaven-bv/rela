package archguard

// unselectedQueryAllowlist pins the EntityQuery literals that set no Faces
// (see unselectedQueries). It may only shrink; it is empty, and a literal
// completed by a helper before it runs is the only kind that may be added,
// with a reason (design A12 of TKT-KQXVF7).
var unselectedQueryAllowlist = map[string]allowed{}

// defaultWorldAllowlist pins the store.DefaultWorld calls outside the store
// and internal/worlds (see defaultWorldCalls). It may only shrink.
//
// Each entry reads identity: who a principal is and what it inherits. Those
// reads keep today's default-world behavior until TKT-7IZHP0 decides, with
// its own security review, how identity reads faced types.
var defaultWorldAllowlist = map[string]allowed{
	"internal/acl/principallookup.go": {1, "principal lookup reads identity in the default world " +
		"until TKT-7IZHP0"},
	"internal/acl/request.go": {1, "the bare-id row gate evaluates a scoped verdict on the default-world " +
		"row until TKT-7IZHP0"},
	"internal/acl/traversal.go": {1, "TraversalQuery walks membership and inheritance in the " +
		"default world until TKT-7IZHP0"},
	"internal/aclmap/enumerate.go": {1, "principal enumeration mirrors the principal lookup's " +
		"default world until TKT-7IZHP0"},
}
