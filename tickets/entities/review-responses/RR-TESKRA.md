---
id: RR-TESKRA
type: review-response
title: guardFailure is sticky across decider reruns
finding: realtime/session.go:513 records only the first failure (failed.err == nil). The pre-check and any backend rerun share failed, so a failure seen in the pre-check is logged even when the run under the lock evaluated cleanly, and a later run's failure on another entry is never reported. Reset *failed at the top of the returned decider, as the plan.marked "assigned, not set" pattern does.
severity: nit
resolution: '*failed was already reset at the top of each decider run; with the fail-closed change, the decider also stops at the first failing guard, so each run records at most its own one failure.'
status: addressed
---
