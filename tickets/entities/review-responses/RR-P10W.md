---
id: RR-P10W
type: review-response
title: Test does not use fluent builder convention
finding: frontend/CLAUDE.md and root CLAUDE.md push fluent builders for tests. The new Go test hand-rolls struct literals with hardcoded IDs. Consistent with existing TestV1Views_* tests in this file but inconsistent with the documented convention.
severity: nit
reason: The new test follows the convention already entrenched in api_v1_test.go (see TestV1Views_DefaultViewForUnconfiguredType and TestV1Views_ConfiguredViewForType immediately above it, which both use the same struct-literal style with seedEntity / seedRelation helpers). Migrating the dataentry test suite to fluent builders is a project-wide concern best handled in a dedicated test-cleanup pass, not bolted into a refactor ticket. If a builder pass happens for this file, this test will be folded in alongside its neighbours.
status: wont-fix
---
