---
id: RR-HIR5U9I
type: review-response
title: No test ties the real fsstore to entityIDLister
finding: The type assertion in collectAllIDs could silently stop matching FSStore or its Tx view after a rename / bringing back the full scan with every test green.
severity: significant
resolution: idlister_fsstore_internal_test.go asserts FSStore implements entityIDLister at compile time and that the Tx view passes the type assertion.
status: addressed
---
