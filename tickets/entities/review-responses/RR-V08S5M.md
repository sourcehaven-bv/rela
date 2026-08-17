---
id: RR-V08S5M
type: review-response
title: Scan and index disagree on non-string unique properties
finding: 'checkUniqueProperties collects values via e.GetString(name), which returns "" for non-string values, so a unique:true integer/boolean property is treated as always-empty → exempt → the app scan never checks it. But the DB index over properties->>''prop'' stringifies JSON numbers and DOES enforce. Nothing restricts unique:true to string types. Result: the scan passes a duplicate integer key, then the index rejects it atomically — a 422 the operator was told the scan catches first; and uniqueViolators counts integer dups the scan thinks don''t exist. FIX: constrain unique:true to string-typed properties at metamodel load (cleanest; unique keys are strings in practice), or make the scan stringify non-string values to match ->>.'
severity: significant
status: open
---
