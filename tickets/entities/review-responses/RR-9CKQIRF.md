---
id: RR-9CKQIRF
type: review-response
title: Received hints must never be sent again
finding: A send-off flag on Local would race with concurrent local writes; EntityRenamed would send moved hints for ephemeral typing drops.
severity: minor
resolution: Internal mark functions take a send argument; the receive path and the EntityRenamed typing drops pass false.
status: addressed
---
