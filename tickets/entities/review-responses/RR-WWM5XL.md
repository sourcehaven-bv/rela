---
id: RR-WWM5XL
type: review-response
title: 'List filter porting: commit to ''apply after'', document that $-variable filters are silently skipped in command payloads'
finding: |-
    The plan says list filters can be ported 'apply after, or translate into query' — the hand-wave hides two real issues.

    1. scopedSortedEntities' own filter pass is driven by the URL wire grammar (filter[key]op=val, applyV1Filters), a DIFFERENT representation from listCfg.Filters ([]FilterConfig{Property,Operator,Value}, applied by applyFilters in helpers.go). applyFilters supports only =/!=; applyV1Filters supports a richer operator set. 'Translate into query' means round-tripping config filters through the wire grammar — non-trivial and lossy. COMMIT to 'apply applyFilters after scopedSortedEntities' (scope+sort from the seam, then the command path's own filter pass), and drop 'or translate'.

    2. applyFilters (helpers.go) SKIPS $-prefixed filter values (variable substitution). The command path has no request context to substitute from, so a listCfg filter like `owner = $currentUser` is silently a no-op in a command payload — the script receives ALL owners' visible rows, scoped only by ACL. This is NOT a regression (today's handleCommandExec calls the same applyFilters at commands.go:426), but it becomes security-relevant once the payload is a permission-gated capability: if the ACL policy does not itself scope by owner, the command sees broader data than the on-screen list implies. The migration note MUST state: 'config filters using $ variables are not applied to command payloads; scope such commands via ACL, not via the list filter.'

    ORDERING (verified clean): scopedSortedEntities scopes via ReadQuery BEFORE sorting (applyV1Sorting runs last on the already-ACL-filtered set), so D2's list-sort adoption does not leak ordering info about hidden rows — they are gone before the sort. That axis is safe.
severity: significant
resolution: 'Addressed in PLAN-Z2OIV7 Approach (list context). Committed to ''apply applyFilters after scopedSortedEntities'' — the ''or translate into query'' hand-wave is dropped. The $-variable filter caveat is documented: applyFilters skips $-prefixed values (no request context to substitute from in a command payload), which matches today''s handleCommandExec behavior but becomes security-relevant once the payload is a permission-gated capability — so the migration note states ''config filters using $ variables are not applied to command payloads; scope such commands via ACL, not the list filter.'' The finding''s clean-ordering observation (scopedSortedEntities scopes via ReadQuery before sorting, so D2''s list-sort adoption does not leak ordering info about hidden rows) is confirmed and recorded.'
status: addressed
---
