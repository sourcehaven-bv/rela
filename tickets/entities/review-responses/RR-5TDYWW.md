---
id: RR-5TDYWW
type: review-response
title: on_absent.redirect only rejects the one-hop self-loop; a→b→a loops the browser forever
finding: validateWorldOnAbsent (internal/metamodel/loader.go) rejects `a → a` only. Two worlds redirecting to each other pass load; in EntityDetail.vue the worldAbsent watcher pushes a route per hop, so the SPA bounces between them indefinitely and floods history. Needs a cycle walk at load and a client-side guard.
severity: critical
resolution: '[security] validateWorldOnAbsent walks the redirect chain with a visited set and refuses any return to a visited world (self-loop included), naming the loop (`published → roundtrip → published`); `default` ends every chain. TestWorlds_OnAbsentRedirectValidated covers the two-world cycle and a legitimate chain. Client belt-and-braces in EntityDetail: a per-entity visited set of redirect sources and targets bounds a loop to one hop and suppresses a duplicate push on schema reload; the watcher now fires per loaded view (not on the absent flag), so scope-nav between two absent entities also redirects.'
status: addressed
---
