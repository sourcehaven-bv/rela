---
id: RR-D3OH9M
type: review-response
title: TestSweepConfigCrossesPackagesUnchanged was tautological
finding: The test claimed to pin the alias decision (pgstore.SweepConfig = store.SweepConfig rather than a named type) but lived in backendneutral_postgres_test.go, which deliberately does not import pgstore. It therefore compared a store.SweepConfig value to itself — true under any definition of the pgstore type, including the named type it was supposed to forbid. It could never fail.
severity: significant
resolution: 'Deleted it and put compile-time assertions in widen_assertions_postgres_test.go, which does import pgstore: bidirectional assignability between store.SweepConfig and pgstore.SweepConfig, plus one on ProjectionProvider. Verified by regression: converting the alias to a named type breaks the build at those exact lines in both directions. A named type would need explicit conversions, so the failure is a compile error rather than an assertion — the loudest available signal.'
status: addressed
---
