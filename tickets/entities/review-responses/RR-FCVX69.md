---
id: RR-FCVX69
type: review-response
title: listFromStoreByTypes error refactor ripples to 3 unplanned callers
finding: The plan conflates 'executeQuery gains error' with 'listFromStoreByTypes gains error'. listFromStoreByTypes has four callers (app.go:519, commands.go:322, handlers_api.go:861, helpers.go:406); a signature change ripples to three the plan ignores, and without it the dropped iterator error remains on executeQuery's non-free-text branch.
severity: significant
resolution: 'Plan rev 2: listFromStoreByTypes is NOT refactored — its 4 callers stay untouched. executeQuery''s non-free-text branch gets its own inline error-surfacing iteration (mirroring the VMD8 AllowAll fix). Other 3 callers explicitly listed as out of scope.'
status: addressed
---
