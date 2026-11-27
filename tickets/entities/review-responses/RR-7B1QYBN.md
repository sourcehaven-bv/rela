---
id: RR-7B1QYBN
type: review-response
title: "check-forwarding misses defineModel props"
finding: "scripts/check-forwarding.mjs reads only defineProps, so a wrapper that drops v-model on a converted child goes unnoticed."
severity: nit
reason: "No wrapper is affected today; extending the script is separate tooling work."
status: deferred
---

scripts/check-forwarding.mjs reads only defineProps, so a wrapper that drops v-model on a converted child goes unnoticed.
