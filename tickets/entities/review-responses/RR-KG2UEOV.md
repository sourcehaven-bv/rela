---
id: RR-KG2UEOV
type: review-response
title: endpointTypeRefusal mapping has no test
finding: Phase A always refuses first so the manager-refusal mapping never ran in tests.
severity: minor
resolution: TestEndpointTypeRefusal covers a bare manager refusal, one wrapped in RelationCreateError, and an unrelated error.
status: addressed
---
