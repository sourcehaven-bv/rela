---
id: RR-OR6LOZ
type: review-response
title: Report says nothing was written after a failed cleanup
finding: The problem header claimed nothing was written even when cleanup failed or the process was killed.
severity: nit
resolution: Header now says the import did not complete; cleanup failures are warnings; the guide mentions the staging directory a killed run leaves.
status: addressed
---
