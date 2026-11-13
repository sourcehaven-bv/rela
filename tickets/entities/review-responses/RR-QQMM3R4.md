---
id: RR-QQMM3R4
type: review-response
title: ScopedHeadersIsDebt test passes when the key disappears
finding: The ok && guard made the test pass on a rename or removal.
severity: minor
resolution: The test now fails when the entry is missing and says to delete the test with the scan in TKT-FED6FMM.
status: addressed
---
