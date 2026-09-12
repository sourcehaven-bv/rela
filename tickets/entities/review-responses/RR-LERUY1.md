---
id: RR-LERUY1
type: review-response
title: Failed dedup query files a duplicate issue
finding: 'The dedup lookup used `|| true`, so a failed `gh issue list` (rate limit, 502, network blip) and a genuine no-match were both the empty string. The script read failure as "no match" and filed a new issue for a target that already had an open one. Since this is the single call implementing dedup, its failure mode defeated the whole point of the change: a blip on iteration 4 of 11 would rebuild issue #993''s problem, spread across duplicate threads instead of one.'
severity: critical
resolution: 'The query now runs under `if ! existing="$(...)"` capturing stderr, so a failure is distinguishable from an empty result. On failure the step warns with the target name and the gh error, increments file_failures, and `continue`s without filing. Fails closed: a skipped comment costs one week of signal on an already-tracked bug; a duplicate costs manual triage every week after. Reproduced before the fix (stub returning exit 1 produced DUPLICATE-CREATE) and verified after (warning + exit 1, no create), including that other targets in the same sweep still file.'
status: addressed
---

Found by cranky-code-reviewer on the TKT-8LZGME diff.

Reproduced against the real step body with a stubbed `gh`:

```
### before fix: dedup query FAILS -> duplicate?
  DUPLICATE-CREATE: Fuzz failure: FuzzGenerateShortID (internal/entity)

### after fix
WARNING: could not query existing issues for Fuzz failure: FuzzGenerateShortID (internal/entity): gh: API rate limit exceeded
ERROR: 1 target(s) could not be filed — see warnings above.
exit=1
```

Isolation also verified: with only `FuzzA`'s lookup failing, `FuzzB` and `FuzzC`
still filed and the step exited 1.
