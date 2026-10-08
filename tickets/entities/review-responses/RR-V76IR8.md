---
id: RR-V76IR8
type: review-response
title: External id accepts Unicode format characters
finding: Bidi overrides and zero-width chars pass validation and spoof display.
severity: minor
resolution: validateExternalID rejects format and non-printable characters (U+202E, ZWSP, BOM, NBSP rows).
status: addressed
---
