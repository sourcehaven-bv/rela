---
id: RR-F5P1L
type: review-response
title: Hardcoded status strings across specs will drift
finding: Specs hardcode 'draft','approved','done','in_progress' etc. Metamodel lives in one place in fixtures.ts but specs don't reference it. Extract to a constants module surfaced from the fixture.
severity: critical
resolution: Exported STATUS, SEVERITY, PRIORITY constants from fixtures.ts. Migrated dashboard.spec.ts to use them. Existing specs can migrate opportunistically; the fixture now owns the enum values so a schema change invalidates only one file.
status: addressed
---
