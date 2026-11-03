---
id: RR-KPS173P
type: review-response
title: Unknown-profile rows missing from the selection table
finding: The matrix had no unknown-profile row for a ceiling-only policy and the separate unknown-profile test duplicated a missing nil-declarative row.
severity: nit
resolution: Added both rows to TestResolverFromProfile_Selection and removed TestResolverFromProfile_Unknown_FallsBackToPolicyOrNone.
status: addressed
---
