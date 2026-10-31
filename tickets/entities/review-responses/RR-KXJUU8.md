---
id: RR-KXJUU8
type: review-response
title: The E2E fixture cannot set composePreset through decisionTest
finding: 'The plan adds `decisionTest("a preset that needs text opens the composer prefilled")` inside describe("session decisions"). `decisionTest` hard-codes `writeRoomProject(testProject, { decisions: true })` (e2e/tests/channels.spec.ts:220-226), so the new flag never reaches the server. Plan a second fixture, such as `composeDecisionTest = base.extend` with `{ decisions: true, composePreset: true }`, or an option fixture. Also make `roomRealtimeYaml` refuse composePreset without decisions, as it refuses the other combinations (e2e/tests/realtime-project.ts:129-131).'
severity: minor
resolution: 'ADDRESSED New composeDecisionTest fixture with { decisions: true, composePreset: true }; roomRealtimeYaml throws on composePreset without decisions (AC 12, Test Plan).'
status: addressed
---
