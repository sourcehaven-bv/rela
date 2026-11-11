---
id: RR-RHE9GW
type: review-response
title: Test reader lacks WithFamilies
finding: retireIDs built the test PolicyReader without the families option the production view reader takes, so face ordering in the tests differed from production.
severity: minor
resolution: retireIDs and the failing-lookup test pass familiesOption(app), as app.go wires the view reader.
status: addressed
---
