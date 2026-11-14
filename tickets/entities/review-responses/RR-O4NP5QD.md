---
id: RR-O4NP5QD
type: review-response
title: Docs and test nits
finding: stub.name and stub:count() were not listed; the WithTimeout comment did not mention a past deadline; tests lacked pattern anchoring and an error inside a sequence; a generated name can collide with a later explicit name.
severity: nit
resolution: 'Godoc lists both and the past-deadline case; tests cover anchoring and an error in a sequence. The name collision keeps its error, which names where the first stub was registered. The 100-round cap of t.flows.work is not tested: it needs a flow that queues work forever at one instant.'
status: addressed
---
