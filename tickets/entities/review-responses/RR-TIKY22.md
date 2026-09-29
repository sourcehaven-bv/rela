---
id: RR-TIKY22
type: review-response
title: Unreachable code after t.Skip
finding: The skipped tests kept their bodies after t.Skip.
severity: nit
resolution: The bodies moved into assert helpers called per fixture; the skip is per subtest, so nothing follows it.
status: addressed
---
