---
id: RR-WG5NY1
type: review-response
title: AC3's no-match-all assertion is vacuous; end-to-end blank-guard at resolver level is untested
finding: 'TestMembershipRelation_Configured_DoesNotFollowMemberOf (resolver_test.go:490-517) asserts g.outgoingByRel[""]==0, but the policy sets MembershipRelation:"heeft_rol" (non-blank). The accessor collapses blank to member-of before the type reaches the fake graph, so in THIS test no code path could ever produce a "" key — the assertion is vacuously true and passes whether or not the guard exists. AC6 tests the accessor in isolation but never asserts the RESOLVER calls it. So the end-to-end property ''a Policy with blank MembershipRelation does not cause the resolver to issue a Type=="" walk'' is asserted nowhere. Fix: add a resolver-level test that builds &Policy{MembershipRelation:"   "} (or unset), runs the actual walk against the fake graph, and asserts outgoingByRel[""]==0 AND outgoingByRel["member-of"]>=1. Fix AC3''s misleading match-all comment (keep its valid member-of-not-followed assertion).'
severity: significant
resolution: 'Added TestMembershipRelation_BlankNeverQueriesMatchAll: builds &Policy{MembershipRelation: blank} over {"", "   ", "\t"}, runs the real resolver walk, and asserts outgoingByRel[""]==0 AND outgoingByRel["member-of"]>=1 AND the editor role resolved. This fails if the resolver stops reading through the accessor — the gap the isolated AC6 test couldn''t catch. Fixed AC3''s misleading match-all comment; AC3 now asserts the positive (walk queried heeft_rol, not member-of) instead of the vacuous outgoingByRel[""]==0.'
status: addressed
---
