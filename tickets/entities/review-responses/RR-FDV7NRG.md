---
id: RR-FDV7NRG
type: review-response
title: Contract event check cannot see a PATCH without an event
finding: entity:changed was compared as one set per run, and every run starts with a create, so a fake that never emitted on PATCH or DELETE passed.
severity: significant
resolution: A quiet op waits out the server's 200ms batch and starts a new segment; events are compared per segment, and the fixed example has a quiet point after its create. A fake without PATCH events now fails.
status: addressed
---
