---
id: RR-FUD017
type: review-response
title: Long-lived Evaluator freezes today(); latent staleness footgun
finding: 'Evaluator captures time.Now() at construction. automation Engine + validation Service build their Evaluator ONCE and reuse for process lifetime, so today() in a predicate would freeze. Not a live bug today: FromFilter never emits today() (it emits literal date strings), and automation/validation only call CompileFilter. But the moment a raw today()-bearing predicate routes through the long-lived Evaluator, `due < today()` silently stops advancing on a long-running server. FIX: make `now` a per-Matches parameter or a func() time.Time, OR document the invariant loudly at the two NewEvaluator(meta, time.Now()) call sites. CLI is fine (fresh Evaluator per command).'
severity: significant
status: open
---
