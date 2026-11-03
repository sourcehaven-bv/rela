---
id: RR-15ETGO
type: review-response
title: E2E leave bound is 45 s not 25 s
finding: presence.spec.ts waits 45 s for bob to leave; AC1 says 25 s so the test does not verify it
severity: significant
resolution: 'Fixed in SPA presence: review fixes'
status: addressed
---
