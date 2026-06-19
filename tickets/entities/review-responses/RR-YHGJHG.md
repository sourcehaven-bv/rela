---
id: RR-YHGJHG
type: review-response
title: Relations require both endpoints to exist — apply order + atomicity unspecified
finding: 'The plan applies pushed/pulled changes through entitymanager but does not specify ORDER or atomicity. Verified in code: CreateRelation (manager.go:664-689) does GetEntity(from) then GetEntity(to) and returns ErrEntityNotFound if either endpoint is missing; it also needs both endpoint TYPES for ValidateRelation (manager.go:684). There is NO transaction boundary across entitymanager calls (entitymanager.go:29-60 has no Begin/Batch/Tx) — each Create/Update commits independently. Consequences the plan must address: (1) a manifest/batch that delivers a relation before one of its endpoints fails hard — the sync apply layer MUST topologically order all entities before any relation that references them; no deferral/forward-ref handling exists. (2) A batch that fails midway leaves a PARTIAL graph on the peer (some entities applied, others not, relations dangling-but-rejected). The plan''s acceptance criteria (''both ends converge'') is not achievable without a defined ordering + partial-failure recovery story (resume from cursor? per-record idempotent retry?). This is the single biggest implementation-blocking gap.'
severity: critical
resolution: 'Plan updated (PLAN-KMAEQQ Approach §5, new AC #6): apply layer topologically orders each batch — all entities before any relation referencing them, relation-deletes before entity-deletes. No batch atomicity exists, so convergence is achieved via per-record idempotent replay (upsert + hash no-op) with the cursor/index advancing only past confirmed-applied records; a mid-batch failure is recovered by re-running and resuming from the last good cursor.'
status: addressed
---
