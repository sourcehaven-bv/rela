package dataentry

// This file exports test internals to package dataentry_test, the way an
// export_test.go would; that name is taken by the export endpoint tests.

// RouterWalkRoute is a route of the router walk, exported for the golden
// corpus in package dataentry_test, whose coverage guard keys on it.
type RouterWalkRoute struct {
	Method, Route, Path string
	// Unwalked says why the router walk skips the route, or is empty.
	Unwalked string
}

// RouterWalkRoutes returns the routes routerWalkProbes lists.
func RouterWalkRoutes() []RouterWalkRoute {
	out := make([]RouterWalkRoute, 0, len(routerWalkProbes))
	for _, p := range routerWalkProbes {
		out = append(out, RouterWalkRoute{Method: p.method, Route: p.route, Path: p.path, Unwalked: p.skipWalk})
	}
	return out
}
