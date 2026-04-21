---
id: RR-CH13R
type: review-response
title: V1EntityType emits both id_prefix and id_prefixes — forever liability
finding: The schema response now emits both id_prefix (string) and id_prefixes (array). The frontend's prefixOptions falls back to singular if id_prefixes is absent. The dual field will desync at some point.
severity: minor
reason: Already designed for back-compat and pinned by TestV1Schema_SinglePrefix_Compat (asserts both fields are populated for single-prefix types). The frontend reads id_prefixes first and only falls back when absent — so for new code there's a single source of truth. Removing id_prefix would be a breaking API change for any consumer reading it directly; deferring that to a future major version. The current shape is the most ergonomic transition path.
status: wont-fix
---
