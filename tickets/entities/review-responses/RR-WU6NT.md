---
id: RR-WU6NT
type: review-response
title: 'Cross-field PATCH ordering: per-field chaining isn''t enough when automations create cross-field side effects'
finding: |-
    Plan section 6 chains PATCHes per-field via .then(). But cross-field there's no chaining. Scenario: user toggles checkbox X (PATCH A in-flight, slow), then types in field Y (PATCH B sends 800ms later). PATCH A's automation may set Y as a side effect; PATCH B then overwrites with stale-by-1.5s typed value.

    Fix: queue PATCHes through a single per-entity FIFO promise chain (not per-field). PATCH B for field Y waits for PATCH A for field X to resolve before sending. One global indicator instead of per-field. Document tradeoff: a single slow request stalls all other fields' saves.
severity: significant
resolution: 'useAutoSave uses a single per-entity FIFO promise chain — every save (any property, content, unset) appends. New calls await the chain''s tail. One global indicator instead of per-field. Plan documents the tradeoff: a single slow request stalls all saves. AC #4 verifies via Vitest with a delayed PATCH A and queued PATCH B.'
status: addressed
---
