---
id: RR-H7SFLV
type: review-response
title: Decision memo key omits session.transitions
finding: useSessionDecisions.ts:78-87 rebuilds views only when the JSON key changes. decisionViews reads `head.channel.session.transitions` (edges, `guarded`, `by`), which the key leaves out. A server config reload that changes a state's edges without changing generation, _actions or the descriptor object would leave stale opensForm, toLabel or confirm values. Add `cs?.transitions` to the key. It is cheap.
severity: nit
resolution: Added cs?.transitions to the decision memo key.
status: addressed
---
