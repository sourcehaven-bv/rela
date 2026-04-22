---
id: RR-65GPQ
type: review-response
title: No helper to write entities directly from tests
finding: If a spec wants to write an entity file directly (e.g., to test the watcher), it must rebuild the path manually. Expose writeEntityFile helper on the test project.
severity: nit
reason: No current test needs a writeEntityFile helper. Adding one speculatively isn't worth the maintenance cost; the first test that needs it can add the helper.
status: deferred
---
