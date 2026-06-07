package acl

// DepthCap bounds every transitive walk the resolver performs
// (member-of for groups, inherit_roles_through for containment). It is
// a safety backstop, not the primary termination mechanism — visited-set
// dedup terminates correctly on cycles and self-loops regardless.
//
// The value 5 is chosen empirically: more than any realistic
// organization or containment hierarchy, while bounded enough to make
// pathological cycles cheap to abort. Operators who hit the cap should
// file a follow-up ticket so the change is considered in context
// (wallclock, audit-debugging) rather than reflexively bumped.
//
// Exported (RR-AROE) so downstream callers — chiefly
// internal/store/graphquerynaive when invoked from readQuery — observe
// the same backstop instead of declaring a parallel constant that can
// drift silently.
const DepthCap = 5

// depthCap is the unexported alias kept for the resolver's existing
// call sites. New code outside this package should reference
// [DepthCap].
const depthCap = DepthCap
