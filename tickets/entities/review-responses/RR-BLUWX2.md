---
id: RR-BLUWX2
type: review-response
title: Required file or external_ref target field makes the recipe unusable
finding: requiredFields asks for file/external_ref fields; a text value is refused by rejectFileCreate/rejectExternalRefCreate and leaving it empty is a hard 422. Type conversions refuse this at load; task-item conversions do not.
severity: significant
resolution: The validator refuses a target with a required file or external_ref property, and refuses mapping a file property. Pinned by three validation cases.
status: addressed
---
