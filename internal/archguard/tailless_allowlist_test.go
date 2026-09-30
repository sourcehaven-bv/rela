package archguard

// taillessKeyAllowlist pins every tail-less entity.RelationKey literal in
// non-test code (see taillessKeys), per repo-relative file. It may only
// shrink. A tail-less key addresses the identity edge; each entry says why
// that edge is the right one.
var taillessKeyAllowlist = map[string]allowed{
	"internal/acl/storegraph.go": {1, "HasEdge: membership edges are identity-scoped, and this " +
		"is the ACL's default-world seam (TKT-7IZHP0)"},
}
