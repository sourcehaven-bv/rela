package archguard

// defaultNameAllowlist pins every reference to the default world's name
// outside defaultNameHomes (see defaultNameRefs), per repo-relative file. It
// may only shrink.
//
// The remaining sites are HTTP world selection and the ACL world grant, which
// still treat "default" as a world every request may name. PR 5b binds the
// HTTP surface to worlds.Compiled (D2, `?world=default` refused beside
// declared worlds); PR 6 moves the ACL grant rule. Each entry names which.
var defaultNameAllowlist = map[string]allowed{
	// dataentry: HTTP world selection, PR 5b.
	"internal/dataentry/world.go": {4, "effectiveDefaultWorld with no schema loaded, " +
		"defaultWorldScope's lookup for a WorldLookup without DefaultWorld, and provenance " +
		"on an unstamped context"},
	"internal/acl/policy.go": {1, "Policy.DefaultWorld before ValidateAgainstMetamodel " +
		"has supplied the schema's"},
	"internal/dataentry/entityref.go": {1, "an entity link's world name defaults to \"default\". " +
		"PR 5b"},
	"internal/dataentry/nextaction_handler.go": {1, "the browsing world of a next-action request. " +
		"PR 5b"},
	"internal/dataentry/write_handler.go": {1, "create body `world: \"default\"`; becomes 400 " +
		"unknown_world beside declared worlds (design 10.5). PR 5b"},

	// dataentryconfig: runtime matching, not validation.
	"internal/dataentryconfig/nextaction.go": {2, "VisibleInWorld compares the browsing world, " +
		"which PR 5b names. PR 5b"},

	// acl: the world grant rule, PR 6.

	// aclaudit explains the reserved name; it does not resolve it.
	"internal/aclaudit/tier_b.go": {6, "B10 names the default world to explain why a grant on " +
		"it, or a miscased spelling of it, matches nothing"},

	// docs: the ACL world matrix states the grant rule above.
	"internal/docs/resolvers_acl.go": {2, "worlds_matrix shows the default world as granted to " +
		"every role, as the ACL rule does. Moves with PR 6"},
}
