---
id: RR-FVCHUA
type: review-response
title: 'rename_entity_type underspecified: renametype delegation is wrong and fsstore UpdateEntity orphans the old file'
finding: 'Two problems. (1) internal/renametype rewrites schema.yaml as part of its job — but at migration time schema.yaml has ALREADY been changed by the operator; delegating to it would double-apply or fail. Only its entity-file/dir rewrite logic is reusable. (2) A ''store-side type rewrite'' via UpdateEntity does not work on fsstore: updateEntity (internal/store/fsstore/entity.go:263) writes the entity at the path derived from the NEW type but never removes the file under the OLD type''s directory — the index is updated but a stale duplicate file remains on disk (and would resurrect on reload/watch). The step needs a dedicated per-backend implementation, not naive UpdateEntity.'
severity: significant
resolution: 'Amendment A3: rename_entity_type is a dedicated step — internal/renametype is NOT delegated to (it rewrites schema.yaml, which has already changed at migration time; its entity-rewrite logic is reference only). The fsstore updateEntity orphan-file bug is fixed as part of this ticket: type-change-on-update becomes a store contract (file relocated when Type differs), pinned by a new storetest conformance case across all backends. AC11 added.'
status: addressed
---
