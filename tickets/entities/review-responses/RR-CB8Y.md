---
id: RR-CB8Y
type: review-response
title: Consider a consumer-side readGate interface to centralize read-gate decisions
finding: The PR wires ACL into 4 read paths now (list, GET, sidebarCount, _position) and TKT-VQGN's follow-ups will add 3+ more (/_search, /_events, includes once the include-bypass is fixed). The current plan threads `acl.FromContext(ctx)` + switch dispatch into each handler. Consider a small consumer-side interface in internal/dataentry — `type readGate interface { Visible(ctx, type, id) bool; Query(ctx, type) ReadQueryResult }` — with one production impl wrapping *acl.Request and one NopGate. Handlers receive readGate via ctx; every read path looks identical. This is what the consumer-side-interfaces rule in CLAUDE.md exists for; the current plan scatters acl.FromContext checks across N handlers, which is the producer-coupling smell the rule prevents. Optional but worth weighing now before the pattern multiplies.
severity: minor
reason: Moved to TKT-VMD8 as optional-but-recommended. By PR 2 the pattern is 4+ call sites and the consumer-side interface pays off. TKT-VQGN's 2 call sites (GET + writes + includes) don't justify the abstraction yet.
status: deferred
---
