---
id: RR-OWA3IR
type: review-response
title: No test checks that the map agrees with runtime enforcement
finding: The aclmap tests check the map and the dataentry tests check the runtime; nothing ran one policy through both.
severity: significant
resolution: TestWebhookCeiling_MapAgreesWithRuntime runs five policies through MapWebhook and two real deliveries and asserts check OK equals 'both deliveries succeed and the second finds the first'. It showed that a redacted match property does not break the find, so the HiddenMatch check was removed (follow-up TKT-3EUSCH).
status: addressed
---
