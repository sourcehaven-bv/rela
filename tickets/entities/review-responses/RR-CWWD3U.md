---
id: RR-CWWD3U
type: review-response
title: The existing DropStoreIndex workaround is not mentioned
finding: 'app.FSFactory.DropStoreIndex and its call in cmd/rela-server/serve.go retire() exist because of this bug (the godoc describes it: a store closing after another store wrote files leaves an index that looks fresh). The plan''s Files to modify omit both. Whatever the outcome, the plan should say whether DropStoreIndex is removed (preferred once the fix holds, with the appbuild no-op variants for sqlite, memory and postgres) or kept, and fix its godoc either way.'
severity: minor
resolution: 'Plan updated: DropStoreIndex in internal/app and internal/appbuild and its call in serve.go retire() are removed.'
status: addressed
---
