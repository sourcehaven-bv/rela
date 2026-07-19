---
id: badge-style-type-key-test
type: automated-measure
title: 'Test: Badge resolves styles via the property''s custom-type name (property ≠ type)'
description: 'Badge.test.ts case where the property name differs from its custom-type name (e.g. property status of type ticket-status): asserts the configured color class is applied instead of badge--gray, plus the property-name fallback for inline enums.'
kind: test
location: frontend/src/components/common/Badge.test.ts, frontend/src/stores/schema.test.ts
status: active
---

Unit test in `Badge.test.ts` (plus schema-store coverage for `styleFor`) where
the property name differs from its custom-type name — e.g. property `status` of
type `ticket-status` with `styles: { ticket-status: ... }` — asserting the badge
gets the configured color class, not `badge--gray`. Also covers the fallback
path where styles are keyed directly by property name (inline enums).
