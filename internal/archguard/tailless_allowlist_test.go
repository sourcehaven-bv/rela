package archguard

// taillessKeyAllowlist pins every tail-less entity.RelationKey literal in
// non-test code (see taillessKeys), per repo-relative file. It may only
// shrink. A tail-less key addresses the identity edge; each entry says why
// that edge is the right one.
var taillessKeyAllowlist = map[string]allowed{
	"internal/acl/storegraph.go": {1, "HasEdge: the ACL walks membership and role relations only, " +
		"and Policy.ValidateAgainstMetamodel refuses a content-scoped one, so the identity edge is " +
		"the only edge it can mean"},
}
