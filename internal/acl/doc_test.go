// Package acl_test holds the test surface for the authorization
// implementation. It is split into two layers, kept in separate files
// so the boundary is visible at a glance:
//
//	features_test.go  — design-property tests. One TestFeature_UC* per
//	                    use case. Each test reads as a story (docstring),
//	                    a graph fixture (builder block), and a small set
//	                    of assertions over three primitives: allow,
//	                    visible, attribution. A failing feature test
//	                    means the implementation drifted from a
//	                    load-bearing claim of the ACL design.
//
//	declarative_test.go / policy_test.go — unit tests. Edge cases
//	                    (cycles, depth-cap boundaries, unstamped
//	                    principals, deny-body shape, multi-relation
//	                    dedup), error paths, formatter details, and
//	                    anything else that's implementation rather
//	                    than design.
//
// Why two layers, not one:
//
// The ACL's load-bearing claims — "groups confer roles transitively",
// "containment inheritance lets a folder grant on its documents",
// "multi-source attribution surfaces every path that opened the gate" —
// are not single-call invariants. They emerge from the composition of
// resolver + policy + graph. A unit test of any single resolver method
// (member-of walk, role attribution, write grant check) can pass while
// the composition silently breaks; the feature tests catch the
// composition.
//
// Conversely, the design properties say nothing about how the resolver
// behaves on a self-loop in `member-of`, a malformed principal, or a
// policy with conflicting role names. Those are implementation concerns
// that get pinned per-bug as unit tests.
//
// The two layers are complementary: unit tests prevent implementation
// bugs; feature tests prevent design drift.
//
// # Reading a feature test
//
// Each TestFeature_UC* has a Go docstring that names the scenario, the
// actor, and what they're trying to do. The test body builds a small
// world (one policy YAML block + a handful of entities and relations),
// then makes assertions through the helpers in testutil_test.go.
//
// Assertions are written in three primitives:
//
//   - AssertAllow / AssertDeny — gate a single op on a single subject.
//   - AssertVisible / AssertContains / AssertHidden — what entities of
//     a given type the actor sees on a list-style read.
//   - AssertPrimarySource / AssertAttribution — which Source values
//     surface in the role attribution chain.
//
// No filtered_by_acl counts, no audit-summary strings, no
// store.GraphQuery shape assertions. Those are implementation details
// the feature tests intentionally do not pin.
package acl_test
