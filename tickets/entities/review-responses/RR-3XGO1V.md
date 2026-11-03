---
id: RR-3XGO1V
type: review-response
title: E2E does not check that the preset reaches the server
finding: 'e2e/tests/channels.spec.ts:255-259 asserts only the session move and the feedback text. In the fixture, topic is optional and revise is unguarded, so the test also passes if the SPA drops the preset entirely. Assert that the posted message carries Topic: Rollback plan (for example via its rendered props), or make topic required in the fixture.'
severity: minor
resolution: "addressed\tTopic is now required in the E2E fixture, so the server refuses the Send and the session does not move unless the SPA adds the preset."
status: addressed
---
