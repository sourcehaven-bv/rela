package archguard

// aclModeAllowlist pins the type switches and assertions on acl.NopACL and
// acl.ReadOnlyACL (see aclModeChecks). It may only shrink.
var aclModeAllowlist = map[string]allowed{
	"internal/access/mode.go": {4, "ModeOf, the one classifier every mode decision goes through"},
	"cmd/rela-server/main.go": {1, "shouldWarnNoACL picks a startup warning, not a request decision; " +
		"it moves to access.Mode when the server builds access.Config (TKT-WFS778 PR 6)"},
}
