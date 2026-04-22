---
id: RR-VLCKA
type: review-response
title: Renderer dispatch duplicates cfg.Script != '' check in 4 places
finding: dispatch in doRender (document.go:175), cache-skip in doRender (:194), availability guard in renderScript (:219), handler GetCached skip (api_v1.go). Four emptiness checks vs. a discriminator enum. Not fragile today because validation guarantees mutual exclusion; fragile to adding a third renderer.
severity: nit
reason: String-emptiness is safe given the validator invariant (config-load-time mutual exclusion). The four checks are local and self-explanatory. Revisit when a third renderer appears — that's the natural forcing function for introducing an enum.
status: wont-fix
---

From go-architect review finding #5.

Won't fix for this ticket. String-emptiness is safe given validator invariant.
Address when a third renderer (e.g., PDF export) appears — that's the forcing
function.
