---
id: RR-PHSTCK
type: review-response
title: 'The loading phase has no exit and no visible cancel'
finding: "phase initializes to 'loading' and the only transition out is the single loadRelations call from the watch. There is no timeout and no cancel: the Cancel button lives inside the `phase === 'choosing'` branch, so during a hung fetch the only exits are the header close button and Escape — and Escape is inert then (see RR-FOCRST). A slow or hung relations read leaves the user on 'Loading relations…' with effectively one way out."
severity: significant
resolution: Cancel now renders in the loading and failed phases, not only when choosing. Combined with the focus fix, Escape also works throughout.
status: addressed
---

## Suggested resolution

Render the actions row (at least Cancel) in the loading phase too, and fix the focus ordering so Escape works.
