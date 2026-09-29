---
id: RR-XUW1JB
type: review-response
title: Missing rename test coverage
finding: No tests for a case-only faced rename, a not-found rename, an unfaced rename, or bypass_acl records per face.
severity: minor
resolution: Added TestFamilyRename_CaseOnly and TestFamilyRename_NotFound, and case variants in the dry-run test. Unfaced renames stay covered by the existing rename tests; bypass_acl per-face records come from the shared authorizeFamily helper that the delete path already exercises.
status: addressed
---
