---
id: RR-BAPSQR
type: review-response
title: 'The purge tombstone is returned by ListVersions as a phantom, restorable version'
finding: 'internal/store/sqlitestore/purge.go writes op=''purge'' into entity_versions with an empty type and no content, and the read paths have no op filter — so after a --force-live purge the user sees a version N with an empty type, and GetVersion hands it back as a restorable snapshot that rela restore would apply as an empty entity. store.VersionOpPurge''s own doc describes it as a sweep-suppression marker rather than history, which argues it should not appear in a user-facing timeline at all.'
severity: minor
resolution: 'NOT changed, deliberately. pgstore behaves identically AND pins it: pgstore/purge_test.go:70-73 asserts ''The 2 content versions are gone; only the tombstone remains'', with ListVersions returning exactly one row whose Op is VersionOpPurge. Filtering it in sqlitestore alone would diverge the two backends under a conformance suite whose entire purpose is that they agree, and would fail pgstore''s existing tests. Whether a tombstone belongs in a user-facing timeline is a product decision about the shared contract, so it belongs in its own ticket covering both backends rather than being changed unilaterally here.'
reason: 'pgstore pins the identical behaviour (pgstore/purge_test.go asserts the tombstone is the one row ListVersions returns after a --force-live purge), so filtering it in sqlitestore alone would diverge two backends under a conformance suite whose purpose is that they agree, and would fail pgstore''s existing tests. Whether a sweep-suppression tombstone belongs in a user-facing timeline is a product decision about the SHARED contract; it needs its own ticket covering both backends.'
status: wont-fix
---
