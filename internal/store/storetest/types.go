package storetest

import "slices"

// entityTypes is every entity type the harness writes. A backend whose
// store only accepts declared types (fsstore) declares these in its test
// factory. A harness test that writes a new type adds it here, or that
// backend's conformance run fails with an unknown-type error.
var entityTypes = []string{
	"atype",
	"btype",
	"client",
	"concept",
	"doc",
	"document",
	"epic",
	"evidence",
	"feature",
	"folder",
	"ghost",
	"group",
	"issue",
	"item",
	"memo",
	"node",
	"note",
	"owner",
	"page",
	"person",
	"policy",
	"project",
	"req",
	"requirement",
	"risk",
	"story",
	"t",
	"task",
	"team",
	"test",
	"ticket",
	"user",
}

// EntityTypes returns every entity type the harness writes, sorted. The
// slice is a copy; callers may change it.
func EntityTypes() []string {
	return slices.Clone(entityTypes)
}
