package archguard

// unselectedQueryAllowlist pins the EntityQuery literals that set no Faces
// (see unselectedQueries). It may only shrink; it is empty, and a literal
// completed by a helper before it runs is the only kind that may be added,
// with a reason (design A12 of TKT-KQXVF7).
var unselectedQueryAllowlist = map[string]allowed{}

// unselectedGraphQueryAllowlist pins the GraphQuery literals that set no
// Faces (see unselectedGraphQueries). It may only shrink.
var unselectedGraphQueryAllowlist = map[string]allowed{
	"internal/acl/readquery.go": {1, "the ACL read query is a template; each executor stamps a " +
		"selection onto a copy"},
	"internal/appbuild/queryscopes.go": {5, "a lowered scope is a fragment (four are \"no query\" " +
		"returns); the caller merges it into a selected query"},
	"internal/dataentry/gantt_handler.go": {1, "completed by stampScope before it runs"},
	"internal/dataentry/listpushdown.go":  {1, "completed by stampScope before it runs"},
	"internal/dataentry/queryscope.go":    {1, "a \"no query\" return of the scope lowering"},
	"internal/dataentry/scopedread.go":    {1, "completed by stampScope before it runs"},
}

// trivialScopeAllowlist pins the store.TrivialScope calls outside the store
// and internal/worlds (see trivialScopeCalls). It may only shrink.
//
// Each entry names a site whose trivial scope is the generated default
// world by construction.
var trivialScopeAllowlist = map[string]allowed{
	"internal/appbuild/appbuildtest/fixture.go": {1, "test fixture: compiles no worlds (appbuildtest may " +
		"not import internal/worlds), so its reads serve the trivial default world"},
	"internal/dataentry/world.go": {1, "defaultWorldHandle: the generated default world, for a " +
		"context no request stamped and an App built without worlds"},
}
