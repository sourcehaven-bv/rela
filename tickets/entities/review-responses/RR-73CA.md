---
id: RR-73CA
type: review-response
title: Fail-closed was incomplete — local/ancestor-conferred visible grants still resolved against the live graph
finding: 'The historical marker neutered only outgoingCounts (has_relation / count_relations). But bindingFor → resolveViaDeclarative → req.ForEntity walks the LIVE graph (role_relations HasEdge + inherit_roles_through ancestors) to build the effective role set, which both SELECTS which roles'' visible: blocks apply and feeds has_role. For a drifted-but-live entity, a role newly conferred after capture (e.g. a reassignment) selects an unconditional visible: grant that reveals a field hidden when the version was written — a leak, not requiring anyone to write has_role. Every historical test used NullGraph or a policy with zero role_relations, so the leaking path was structurally untestable.'
severity: critical
resolution: 'Under the historical marker, resolveViaDeclarative resolves GLOBALS-ONLY (skips ForEntity, uses req.Globals attributions). Because the visible: closed-world is role-scoped, dropping to globals-only could leave a reader with zero roles and default to all-visible — so FieldVerdicts now applies a TYPE-LEVEL closed-world under the marker for any type a role gates with visible: (new typesWithVisible set): a field shows only if a globally-held role affirmatively grants it. Regression tests: TestHistoryRedaction_LocalRoleConferred_FailsClosed (dataentry, real StoreGraph + live owns edge), TestHistorical_TypeLevelClosedWorld_EmptyRoleSet + TestHistorical_NoVisiblePolicyForType_MarkerInert (affordances).'
status: addressed
---
