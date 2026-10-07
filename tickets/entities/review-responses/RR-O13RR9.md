---
id: RR-O13RR9
type: review-response
title: Manual-id check not repeated inside the Tx
finding: familyRename.inTx re-authorizes faces that appear late but did not re-run requireManualID.
severity: minor
resolution: inTx calls requireManualID on the re-read family.
status: addressed
---
