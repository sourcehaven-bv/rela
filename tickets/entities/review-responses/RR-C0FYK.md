---
id: RR-C0FYK
type: review-response
title: e2e test uses waitForTimeout instead of waitForResponse
finding: forms.spec.ts uses waitForTimeout(1500) after Save. It races on loaded CI and wastes time on fast boxes. Use waitForResponse on the PATCH URL+method+status or expect.poll on api.getEntity.
severity: minor
resolution: Replaced the 1.5s waitForTimeout with apiPage.waitForResponse() filtered on URL+method, plus a status-200 assertion. Deterministic and fails fast on backend errors rather than silently racing.
status: addressed
---
