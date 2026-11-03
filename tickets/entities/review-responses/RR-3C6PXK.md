---
id: RR-3C6PXK
type: review-response
title: Unreachable `toLabel ?? ''` builds a broken sentence if it is ever reached
finding: useSessionDecisions.ts:152 renders 'This ends the session in .' when toLabel is undefined. Today that cannot happen, because several targets set opensForm and return earlier (line 148). The coupling is implicit. Either set `confirm` only when toLabel is set (channelView.ts decisionViews) or assert it. `confirm` on an opensForm view is also meaningless and could be left unset.
severity: nit
resolution: decisionViews sets confirm only on a one-click decision with a toLabel, so a form decision never asks and the confirm body always has a state name; tests and the fixture were updated, with a new one-click danger case.
status: addressed
---
