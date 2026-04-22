---
id: RR-CKLD2
type: review-response
title: 'return_to collision: rewriter concatenates over author-supplied return_to'
finding: 'rewriteHref at internal/dataentry/document.go:429-433 concatenates the injected return_to onto the raw existingQuery. An author who writes [x](/form/full_ticket?return_to=/evil) produces ?return_to=/evil&return_to=<real>. vue-router resolves duplicate keys as an array; DynamicForm.vue:138 assigns it to returnTo.value and the subsequent .startsWith(''/'') check (line 354) will throw or misbehave on a non-string. Same class via Lua: rela.url("/form/x", {return_to="/evil"}) silently produces ?return_to=/evil and the rewriter appends a second one. Latent redirect-smuggling primitive. Fix: strip any pre-existing return_to in the rewriter before injecting; reject return_to as a reserved key in mergeParamsTable.'
severity: critical
resolution: 'Two-layer defense. (1) internal/dataentry/document.go: new stripQueryKey() drops any pre-existing return_to from the existing query before injecting the rewriter''s own value, logging a warning with the offending href. Handles goldmark-emitted &amp; separators alongside literal &. Three table-driven test cases cover plain collision, collision with other params preserved, and collision via &amp;-separated pairs. (2) internal/lua/urls.go: mergeParamsTable now rejects return_to as a reserved key with a clear Lua error, so rela.url(..., {return_to=...}) fails at render time.'
status: addressed
---
