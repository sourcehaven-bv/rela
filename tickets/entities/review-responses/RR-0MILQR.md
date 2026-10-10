---
id: RR-0MILQR
type: review-response
title: Probe test cannot tell a missing download route from a missing file
finding: The {fileName} exemption skipped the stdlib-404 check so a removed download or delete route would pass.
severity: minor
resolution: The probe app seeds shot.txt through the router; the exemption is gone so every operation must reach a handler.
status: addressed
---
