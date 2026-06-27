---
id: RR-GB4UHX
type: review-response
title: Master secret must fail-loud on multi-process backends; per-process-random default is a silent ghost-row bug invisible in single-process test
finding: 'Cross-process correctness (Alice''s REST cacheId on process 1 must equal her SSE-delete cacheId on process 2) requires every process share one master_secret — pgstore/feed.go ships no secret over NOTIFY (by design), so each process computes cacheId from its LOCAL secret. If unset → per-process-random → cacheId_A1 ≠ cacheId_A2 → client never evicts → permanent ghost row. Invisible in single-process dev/test (one consistent random secret), breaks only in the multi-process postgres deployment. Fix per CLAUDE.md ''reject nil required fields, never substitute silently'': postgres build with no secret → refuse start / disable SSE with actionable error; single-process fsstore → per-process-random acceptable but slog.Warn (correlation won''t survive restart). Store as env var (RELA_SSE_SECRET, never a flag — same ps/history rationale as RELA_DATABASE_URL), read once at appbuild/cmd wiring. Master rotation → all cacheIds change → clients re-fetch; no per-principal rotation.'
severity: critical
reason: Moot under the final per-type design. No cacheId → no master secret → no cross-process secret distribution problem. The whole finding existed only for the HMAC cacheId scheme, which was rejected in favor of id-less `{type}` nudges gated on ReadQuery.
status: wont-fix
---
