---
id: RR-PMZWS0
type: review-response
title: Code-review minor/nit batch (7 findings)
finding: 'Batch of minor and nit findings from the code review: (9) rename_property silently dropped the old value on a both-keys conflict; (10) LoadDir swallowed all PathErrors (incl. permission denied) as an empty chain; (11) convert''s `!=` on `any` could panic on future uncomparable coercions; (12) published Verdict payload not documented immutable; (13) printRunResult sliced hashes without a length guard; (14) GC sweep''s first tick delayed a full interval; (15) RELA_DATA_GC=off matched exactly, so typos silently ENABLED the destructive sweep; plus nits: rel: prefix idioms, decodeStrict rationale.'
severity: minor
resolution: 'Fixed in commit bddc13f3: (9) conflict now leaves BOTH values untouched with a note (TestRenameProperty_ConflictLeavesBothValues); (10) only fs.ErrNotExist reads as an empty chain, other errors surface (TestLoadDir_UnreadableDirectorySurfaces); (12) Verdict documented immutable-after-publication; (13) %.12s formatting; (15) kill switch fails safe — only unset/on/1/true keep the sweep running, anything else disables with a log line; nits: strings.CutPrefix in ledger.go, decodeStrict why-comment. Deferred with reasons: (11) currently unreachable (all uncomparable types short-circuit before the comparison) — left as-is; (14) interval-first is deliberate and now documented at the ticker: an immediate tick would make every short-lived CLI command implicitly delete expired drift; deployments whose server never survives an interval run `rela migrate gc` explicitly.'
status: addressed
---
