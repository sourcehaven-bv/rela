---
id: RR-LETMK8U
type: review-response
title: Testdata packages can be tested
finding: A staged file under testdata/ would be tested explicitly although ./... never does.
severity: nit
resolution: Paths under testdata/ are filtered out.
status: addressed
---
