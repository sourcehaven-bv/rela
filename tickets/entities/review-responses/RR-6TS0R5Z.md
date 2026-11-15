---
id: RR-6TS0R5Z
type: review-response
title: No e2e step picks a relation default
finding: e2e/tests/settings.spec.ts only checks that the card is visible; nothing drives the picker against a real server.
severity: minor
reason: e2e suites were not run in this session; adding an unrun spec risks a red CI. Component tests cover the picker and golden tests the wire shape. Follow-up can add the step.
status: deferred
---
