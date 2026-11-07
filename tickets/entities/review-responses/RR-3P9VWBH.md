---
id: RR-3P9VWBH
type: review-response
title: help YAML tag differs between fields and relations
finding: FormField.Help is yaml:help; FormRelation.Help was yaml:help,omitempty.
severity: nit
resolution: FormRelation.Help now uses yaml:help like FormField.
status: addressed
---
