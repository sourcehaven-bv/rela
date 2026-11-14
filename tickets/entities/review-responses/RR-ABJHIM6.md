---
id: RR-ABJHIM6
type: review-response
title: An unreadable or oversized request body escaped the failure rule
finding: readRequestBody errors returned before the call was recorded or the case failed, so a script that swallowed the error passed.
severity: significant
resolution: The call is recorded and the case fails with the read error.
status: addressed
---
