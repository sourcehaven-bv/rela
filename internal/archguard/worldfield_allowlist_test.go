package archguard

// worldFieldAllowlist pins every lua.ReadDeps or mcp.Deps literal that does
// not set World (see worldFieldLiterals), per repo-relative file. It may only
// shrink, and it starts empty: every wiring site names its world.
var worldFieldAllowlist = map[string]allowed{}
