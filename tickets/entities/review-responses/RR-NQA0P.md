---
id: RR-NQA0P
type: review-response
title: internal/lua not gated by coverage floor - original AC 13 was a no-op
finding: '`.testcoverage.yml` has no `^internal/lua$` override, and the project total of 65% is the only gate. The acceptance criterion ''internal/lua coverage stays at floor or improves'' has no floor to stay at.'
severity: minor
resolution: 'Addressed in AC 18: ''add a package floor of 85% to `.testcoverage.yml` as part of this ticket so AC 13 / future regressions are actually gated''. Implementation will include the `.testcoverage.yml` change alongside the code so the floor is established atomically.'
status: addressed
---
