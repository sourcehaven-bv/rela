---
id: BUG-Q60V
type: bug
title: v1 entity create bypasses field-affordance write gate
description: 'handleV1CreateEntity (POST /api/v1/<type>) goes straight to entityManager.CreateEntity with no field-affordance check. A role whose acl.yaml policy denies writing a field (read-only or hidden) or restricts an enum option can set that field''s value at create time, since the gate that the PATCH path enforces via validateFieldWrite is absent on the create path. Collection-level create (acl.OpCreate) is still enforced inside CreateEntity, but per-field write grants are not. Found by tschmits in CISO review of PR #841 (the ''ter kennisgeving'' note). Must be closed before fields: policies go to production.'
priority: high
effort: s
why1: POST /api/v1/<type> (handleV1CreateEntity) called entityManager.CreateEntity without running validateFieldWrite, so per-field write grants were never checked at create time.
why2: When the field-affordance gate (TKT-9E57/TKT-G7N5) was added, it was wired only into the PATCH/update path (handleV1UpdateEntity) and the read/serialize path; the create path was overlooked.
why3: Create and update share the same _fields verdict semantics but had no shared enforcement seam — each handler calls validateFieldWrite independently, so adding it to one did not cover the other.
why4: There was no contract test asserting that every write entry point (create AND update) enforces field affordances; the affordance_contract_test pinned _actions verbs but not per-field create gating.
why5: 'Systemic: write-path affordance enforcement is duplicated per-handler rather than funneled through a single choke point, so a new or existing write path can silently skip the gate. A grep/contract test enumerating write entry points would catch the omission.'
prevention: 'Added MEAS-create-field-affordance-test asserting the create path enforces field affordances with the same 403 + rule_id shape as PATCH. Systemic follow-up (why5): write-path field-affordance enforcement is duplicated per-handler (create vs update each call validateFieldWrite independently); a future refactor should funnel all write entry points through a single affordance choke point, or add a contract test enumerating every write entry point and asserting each gates field affordances, so a newly added write path cannot silently skip the gate.'
status: review
---
