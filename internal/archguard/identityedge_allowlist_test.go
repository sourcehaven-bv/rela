package archguard

// identityEdgeAllowlist pins the files outside internal/acl that resolve an
// identity (see identityEdgeCalls). Each one is a request edge, or a tool that
// is its own edge because it evaluates principals it chose itself. It may
// only shrink; add a file only when it is a new edge, with the reason.
var identityEdgeAllowlist = map[string]allowed{
	"internal/access/opener.go": {2, "Opener.Open: the data-entry HTTP edge, with the " +
		"principal_property lookup; Restamp: re-resolves a provisioned user and opens a new Request"},
	"internal/aclmap/aclmap.go": {1, "who-can analysis: opens a Request per candidate principal " +
		"it enumerates, so it is its own edge"},
	"internal/aclmap/mapprincipal.go": {1, "maps a raw principal to its user entity for the report, " +
		"via the principal_property lookup"},
	"internal/aclmap/whocan.go": {1, "maps a raw principal to its user entity for the report, " +
		"via the principal_property lookup"},
	"internal/appbuild/identity.go": {1, "IdentityBinder: the one binder every graph-identity " +
		"transport (CLI, rela-docs, scheduler, stdio MCP) takes at wiring"},
	"internal/cli/classification_acl.go": {1, "acl audit: opens a Request for a synthetic " +
		"principal per role to compute its static read view"},
	"internal/dataentry/provision.go": {1, "lazy provisioning: binds the system provisioner " +
		"that creates the stub"},
	"internal/dataentry/watcher.go": {1, "SSE stream: re-resolves membership for a long-lived " +
		"connection after a role-conferring relation change"},
	"internal/dataentry/webhook.go": {1, "IdP webhook: outside /api/, so attachACLRequest does not " +
		"run; binds the receiver principal"},
	"internal/dataentry/webhook_routes.go": {1, "operator webhook routes: outside /api/, so " +
		"attachACLRequest does not run; binds the hook's principal"},
	"internal/docs/assert_acl.go": {1, "docs build ACL assertion: binds the documented principal " +
		"it asserts about"},
	"internal/docs/assert_read.go": {1, "docs build read assertion: binds the documented principal " +
		"it asserts about"},
}
