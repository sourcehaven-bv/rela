---
id: RR-VUKJ
type: review-response
title: Generalize the no-third-party-fetch guard to the appPage fixture
finding: The 'self-contained binary' invariant applies to every SPA view, not just the markdown editor. Moving the off-origin-request detector into the appPage fixture (with throw in afterEach) turns every existing e2e test into a regression guard at zero ongoing cost.
severity: minor
resolution: Done — guard now lives in appPage fixture (fixtures.ts). All 192 existing e2e tests now act as regression guards for the self-contained-binary invariant across the whole SPA; verified by running the full suite green.
status: addressed
---
