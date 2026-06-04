---
id: RR-UD1J
type: review-response
title: LoC accounting is fiction
finding: |
  Ticket claims "net negative on EntityDetail.vue, small positive across widgets, total +200/-150." Each of 8 widgets gains script-side mode discriminator + v-if/v-else block + tests; realistic +15-25 LoC per widget = +120-200 across widgets. EntityDetail saves ~15 LoC per delegated block x 3 = ~45 LoC. Net is positive, probably +100 to +150. The win is in cohesion, not LoC. Reframe AC #9 to a concrete goal -- e.g. EntityDetail.vue drops by >= 30 lines -- rather than a misleading global net-negative claim.
severity: minor
resolution: |
  Plan revised. Dropped AC #9 entirely (no LoC target). Dropped the "Why this design" LoC-accounting paragraph. The structural goal -- per-widget rendering logic moved out of EntityDetail.vue into widget files -- stands on its own without quantitative decoration. EntityDetail.vue's max-lines: 500 ESLint check is tracked in the verification gate but not as a hard pass/fail.
status: addressed
---
