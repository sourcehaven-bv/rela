---
id: RR-H3Z72TJ
type: review-response
title: Per-attempt idempotency key cannot deduplicate across retries
finding: The approved key includes the attempt, so a target cannot drop the repeat after a timeout or network error.
severity: significant
resolution: Plan keeps the approved per-attempt key and adds a stable step key (run/state.generation) as rela.params.step_key and X-Rela-Step.
status: addressed
---
