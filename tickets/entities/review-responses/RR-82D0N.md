---
id: RR-82D0N
type: review-response
title: 'cfg.Timeout behavior for script: renders must be specified'
finding: 'DocumentConfig.Timeout is an existing YAML field. Plan''s section 1 retains it but never says whether it applies to script: renders. If unspecified, authors who set timeout: 120 for an expensive script will get 30s truncation from lua.DefaultTimeout.'
severity: significant
resolution: 'cfg.Timeout wired into lua.WithTimeout inside ExecuteDocument. AC11 tests infinite-loop + timeout: 1 terminates within ~2s wall clock.'
status: addressed
---

From design-review on PLAN-78HJO.

Two choices: (a) Wire `cfg.Timeout` into `lua.WithTimeout(...)` for script
renders (consistent with command: timeout semantics; recommended). (b) Document
that Timeout: only applies to command: renders (inconsistent, confusing).

Recommend (a). Add to AC3 or a new AC.
