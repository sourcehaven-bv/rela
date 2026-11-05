---
id: RR-2OKF1P7
type: review-response
title: List dropped parse errors silently
finding: A real parse failure in the list looked the same as a file resolved since the scan.
severity: minor
resolution: The list logs a warning for every parse failure other than a missing file.
status: addressed
---
