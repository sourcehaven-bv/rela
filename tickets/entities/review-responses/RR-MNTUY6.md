---
id: RR-MNTUY6
type: review-response
title: Non-string status replaced by a default
finding: GetString returns empty for a non-string status value, so a declared default overwrites it.
severity: minor
resolution: Not changed.
reason: Older than this change and outside its scope. Status values are strings in every schema form rela accepts, and a non-string status already fails enum validation, so replacing it with the declared default does not lose a valid value.
status: wont-fix
---
