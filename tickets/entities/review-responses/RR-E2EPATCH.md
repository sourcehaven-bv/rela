---
id: RR-E2EPATCH
type: review-response
title: "e2e autosave wait matched any PATCH under /api/v1/"
severity: nit
status: addressed
finding: "toggleSectionCheckbox waited for any PATCH, so on a page with another autosaving control it would resolve on the wrong request and the caller's reload would race the real save — reintroducing the exact flake the helper was added to remove."
resolution: "Narrowed the predicate to include the entity id from the current URL."
---
