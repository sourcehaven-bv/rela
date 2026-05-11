---
id: RR-WZMP
type: review-response
title: Inaccessible-without-title rendering omits the title — URL still uses the ID, but text could include title when available
finding: "frontend/src/utils/markdown.ts line 89: `const linkText = hit.inaccessible ? \\`${token.text} \U0001F512\\` : hit.title`. When inaccessible=true, we always use `<ID> \U0001F512` as link text, even if the title field is non-empty. Yet the only reason title would be empty is encryption — if the server happens to have a title for a partially-locked entity, the SPA throws it away. With the partial-lock concern from RR-47N9, this combines badly: a partially-locked entity with readable title 'Important thing' would render as 'TKT-001 \U0001F512' rather than 'Important thing \U0001F512'. Suggest: `const linkText = hit.inaccessible ? \\`${hit.title || token.text} \U0001F512\\` : hit.title`. Add a test case for it."
severity: minor
resolution: "rewriteEntityRefToken now keeps the readable title alongside the lock when supplied: visibleTitle = hit.title || (hit.inaccessible ? token.text : ''); linkText = inaccessible ? `${visibleTitle} \U0001F512` : visibleTitle. New test 'keeps the readable title alongside the lock when one is supplied' covers the partially-locked case."
status: addressed
---
