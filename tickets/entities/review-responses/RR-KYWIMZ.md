---
id: RR-KYWIMZ
type: review-response
title: AC6 data-destruction guard is theatre — the exact regression it claims to pin passes
finding: 'TestScriptReads_UpdatePreservesHiddenProperties asserts that two struct fields on a hand-built fixture return different things. It never invokes rela.update_entity. Verified independently: pointing luaUpdateEntity at VisibleReader (the precise ''tidy the two fields together'' regression) leaves the whole package green. Worse than no test, because the godoc AND the new CLAUDE.md rule both cite it as the protection. The invariant itself is airtight (luaUpdateEntity is the sole WritePrepStore reader; the nil-check is unreachable on reader runtimes) — only the test is hollow. Rewrite to run rela.update_entity through a real entitymanager and assert the PERSISTED entity still has the hidden property.'
severity: critical
resolution: 'Rewritten to run the REAL binding: the test now executes rela.update_entity through a persisting Mutator and asserts the STORED entity still carries the hidden property. Mutation-verified — pointing luaUpdateEntity at VisibleReader now FAILS the test, where the previous version passed silently. Confirmed independently before and after the rewrite.'
status: addressed
---
