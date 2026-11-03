---
id: RR-JUE8Q5
type: review-response
title: Fixture cannot exercise several risks
finding: 'One when: arm, no related() when:, history not wired, version reads uncounted, too few rows to expose N+1.'
severity: significant
resolution: Plan fixture covers both arms, a related() when:, app.versions, >=3 rows per type; faults stay in failclosed_test.go.
status: addressed
---
