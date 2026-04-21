---
id: RR-KXIBR
type: review-response
title: resolve makes 4 passes over the key string
finding: 'resolve does: rune loop for control chars, ContainsRune for backslash, ContainsRune for colon, HasPrefix, Split. Could be consolidated to a single pass.'
severity: nit
reason: Premature optimization. resolve is only called on state-KV and (future) per-write operations, which are inherently I/O-bound — the string-scan cost is lost in the noise. Readability of the explicit rule-by-rule check is worth more than the microseconds saved. Revisit if profiling shows it on a hot path after fsstore migration.
status: wont-fix
---
