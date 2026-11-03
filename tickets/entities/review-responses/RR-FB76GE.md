---
id: RR-FB76GE
type: review-response
title: Keep the 400 independent of the graph
finding: The gate's answer must depend on config and the caller's own roles only. Refining it with actual local-role edges or running it only when rows exist would make the 400 an oracle. State it in the godoc and run the gate before the page scope and the DenyAll check.
severity: nit
resolution: listPage gates first, before resolveListNarrowing, the page scope, q= and the deny-all check; the godoc of listfieldgate.go and FieldVisibleOnEveryRow states that the answer must not depend on rows. The golden 400s show the same reads for every principal.
status: addressed
---
