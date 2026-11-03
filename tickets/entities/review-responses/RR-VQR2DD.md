---
id: RR-VQR2DD
type: review-response
title: Head budget test cannot detect the growth it claims to pin
finding: 'internal/dataentry/channels_budget_test.go:283-360 (sessionThreadsWith, TestChannelsBudget_HeadSession): both sizes have exactly two transitions out of the current state (s0), and every state uses the same two roles (officer, owner). Because heldRoles memoizes, the call count stays equal for n=1 and n=10 even if fireable walked every state''s transitions or asked roles per transition. The test therefore cannot fail on the regressions AC 6 names. The "each distinct role asked once" half is delegated to TestAffordances_RolesAskedOnce (internal/realtime/session_affordance_test.go:331), which covers only a permission role, not the entity-property role the plan names. Fix: give each state distinct roles (for example officer_i or owner_i from different sources) and make the large case''s current state carry more transitions sharing one role. Then assert the accessor counts (HoldsPermissionFor and the entity reads through acc) per distinct role of the current state only.'
severity: minor
resolution: The head budget test varies transitions out of the current state (1 vs 10) with distinct roles per state; per-transition or other-state asks fail it; a realtime test counts property and permission role asks.
status: addressed
---
