---
id: RR-JVCMDF
type: review-response
title: '[security] No TOCTOU test for faces added mid-delete'
finding: The race between authorization and delete had no test.
severity: minor
resolution: TestFamilyDelete_FaceAddedDuringDeleteIsAuthorized covers both windows on every backend.
status: addressed
---
