---
id: RR-XBMBS
type: review-response
title: 'Test gaps: negative paths, multi-type, scoped-to-type-present semantic'
finding: 'Missing tests: unknown relation type in payload, unknown target id, multi-type payload (each type reconciled independently), pre-existing edge of a type NOT present in the payload stays untouched (the central ''scoped'' semantic is asserted only indirectly), duplicate targets in the same list.'
severity: significant
resolution: Added TestV1UpdateEntity_Relations_ScopedToTypesInPayload (guarding the untouched-type semantic), _MultiType (two relation types in one PATCH), _UnknownType, _UnknownTarget, _SourceTypeMismatch, _OnlyPATCH_ETagChangesButEntityStable, and expanded the happy-path test to cover duplicate-ids-in-list. Test metamodel grew a `blocks` relation to make the multi-type cases meaningful.
status: addressed
---
