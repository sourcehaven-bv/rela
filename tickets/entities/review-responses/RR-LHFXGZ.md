---
id: RR-LHFXGZ
type: review-response
title: Suppressed denial count lost at end of burst; ErrInvalid branch logged unsampled despite docs claiming otherwise
finding: Two interacting issues. (1) The sampler incremented `suppressed` under lock but only flushed it when a LATER request arrived after the interval, so the residual at the end of an outage was never reported — while the const comment claimed occurrences are "folded into the next line", true only if a next line exists. (2) Only the keys-unavailable branch was sampled; the ErrInvalid branch logged at Info on EVERY request. Since classify() deliberately defaults to ErrInvalid, a rotation-during-outage could flood the unsampled branch — and docs/server-security.md claimed denials are "rate-sampled, so an outage cannot flood the log", which was therefore an overclaim.
severity: minor
resolution: 'Extracted a reusable logSampler type with independent per-class counters, so a flood of one class cannot mask the other, and both branches are now sampled. Added jwtGate.noteRecovery, which drains and reports the residual on the first successful verification after a burst — the moment an operator most wants the outage''s size. Rewrote the docs paragraph to state both classes are sampled AND to disclose that classification is heuristic (biased toward a missed alert over a false page). Verified live: a 7-denial burst produced one sampled line plus `suppressed_invalid=7` on recovery.'
status: addressed
---

Reported by cranky-code-reviewer against `internal/dataentry/jwtgate.go:147-161`
and the docs claim in `docs/server-security.md`.

The reviewer noted the two design choices "interact in a way neither comment
acknowledges" — sampling only the Error branch while `classify()` defaults to
`ErrInvalid` meant the flood landed in whichever branch the default picked. That
framing is what made this worth fixing rather than documenting away.

Verified live:

```
INFO msg="jwt gate: verification succeeded after a run of denials"
  suppressed_keys_unavailable=0 suppressed_invalid=7
```
