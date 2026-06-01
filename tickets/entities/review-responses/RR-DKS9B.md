---
id: RR-DKS9B
type: review-response
title: 'Widget name mismatch: ''multiselect'' (frontend) vs ''multi-select'' (Go)'
finding: FieldRenderer checks widget==='multiselect' (no hyphen). Go config constant is 'multi-select'. Either today's frontend silently ignores Go-side multi-select widget config, or there's a config the Go side rejects. Pre-existing bug the registry will surface or paper over.
severity: critical
resolution: 'Plan revised: dedicated sub-task to audit repo configs for ''multiselect'' vs ''multi-select'' usage. Normalize on ''multi-select'' (matches Go side). Document decision in planning checklist before coding. See TKT-MZSIJ ''Widget name normalization''.'
status: addressed
---
