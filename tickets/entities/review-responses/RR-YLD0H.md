---
id: RR-YLD0H
type: review-response
title: No floors on internal/store/{fsstore, memstore} despite high current coverage
finding: 'memstore 96.2%, fsstore 81.45% — both critical persistence layers with no floors. If tests are deleted, 20pp could drop before total floor catches it. Add 70 floor each. Scope: follow-up, not a blocker.'
severity: nit
reason: Scope-small follow-up per reviewer. Current total floor of 65% catches the store-regression case eventually; dedicated store floors are additive and don't block this PR.
status: deferred
---
