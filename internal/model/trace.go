package model

// TraceResult represents the result of a graph trace operation.
// It forms a tree structure showing entities reachable from a starting point.
type TraceResult struct {
	ID       string
	Type     string
	Title    string
	Depth    int
	Relation string // The relation that led to this node
	Incoming bool   // True if this node was reached via an incoming relation
	Children []*TraceResult
}

// PathStep represents a step in a path between two entities.
type PathStep struct {
	ID       string
	Type     string
	Title    string
	Relation string // The relation that led to this step
}
