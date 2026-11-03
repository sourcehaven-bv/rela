---
id: RR-DC3EP7A
type: review-response
title: Hidden form fields reappear as bare inputs
finding: A hidden form field is filtered out and then re-added by the second loop because it is not in seen.
severity: nit
resolution: A hidden optional field stays hidden; a hidden required field is shown without its hidden flag. Tests cover both.
status: addressed
---
