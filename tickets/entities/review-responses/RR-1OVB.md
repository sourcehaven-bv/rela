---
id: RR-1OVB
type: review-response
title: TestV1EntityRelations_OutgoingOrderableSorted doesn't verify the sort fires
finding: |-
    seedOrderableFixture chose IDs (STP-1, STP-2, STP-3, STP-MISSING) whose alphabetical order is identical to the orderable sort order. memstore returns relations alphabetically by file key, so the test would pass even if sortRelationGroup were a no-op. Pre-existing concern I noted in the diff but didn't actually fix.

    File: internal/dataentry/api_v1_test.go:2892-2918 (also TestV1GetRelationType_OutgoingOrderableSorted at 2924-2952 reuses the same fixture).

    Fix: pick IDs whose alphabetical order DIFFERS from the orderable order, e.g. STP-Z=1, STP-A=2, STP-MISSING=nil, STP-M=3.
severity: critical
resolution: Re-seeded with target IDs (Z=1, X=2, M=3, Y=missing) whose alphabetical order (M, X, Y, Z) differs from the orderable order. Disabling sortRelationGroup now fails the test (verified locally). The negative test (TestV1EntityRelations_NonOrderable_NotSortedByOrderProperty) reuses the same fixture; updated to assert the result is NOT in orderable order.
status: addressed
---
