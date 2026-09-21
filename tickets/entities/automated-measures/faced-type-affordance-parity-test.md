---
id: faced-type-affordance-parity-test
type: automated-measure
title: 'Test: every affordance a faceless type offers must be reachable on a faced type'
description: 'Primary measure for BUG-G2BASF. The command gate at EntityDetail.vue:199 was tested for what it suppresses, never for what remains reachable — and on a faced type the answer is nothing, because there is no bare face to fall back to. A render-time gate keyed on servedFace is total for a faced type and partial for a faceless one, so a test using a faceless fixture cannot observe the difference. Pin the parity directly: for each affordance an entity detail page offers (commands, actions, next-action offers), assert it is reachable at some address on a faced type, not merely that the guard fires.'
kind: test
location: frontend/src/components/entity/EntityDetail.world.test.ts, frontend/src/components/NextActionOffers.test.ts
status: active
---

## Why

A gate written as `servedFace ? [] : loaded` reads as a partial restriction —
"not on this face, use another" — and is tested as one. On a **faced** type it
is total: BUG-HC6I2T removed the bare face, so no address satisfies the `else`
branch and the affordance vanishes from the product with no error, no log line
and no failing test.

Nothing connects the gate to the addressing model it depends on. The gate says
"a bare face exists to fall back to"; the model stopped providing one; both
halves still compile and pass.

The test that existed asserted the suppression directly
(`expect(w.text()).not.toContain('Run publish script')`), so it did not merely
miss the defect — it **pinned** it. Replacing that assertion was part of the
fix, and is the clearest evidence for the rule below.

## What it pins

For each affordance an entity detail page offers, a test asserts it is
**reachable at some address on a faced type**. Suppression-only assertions are
not sufficient — they pass identically whether the fallback address exists or
not.

As implemented (BUG-G2BASF):

- `renders operator COMMANDS at a NON-bare face, addressed BY that face (S1)`
— the button renders, and `CommandModal` receives `POL-1@published`, not the
bare id. Asserting the *address* is what makes rendering it honest: a button
that renders while sending the wrong row is the failure the old gate was
(correctly) trying to avoid, and it is the address that removes it.
- `addresses a command by the BARE id when no face is served` — the faceless
case stays exactly as it was.
- `NextActionOffers > action` — the offer renders, runs, honours `confirm`, and
survives a failure.

## Scope note

A parity test, not a snapshot. It does not assert how an affordance looks — only
that a faced type is not silently a second-class surface. The faceless case was
already covered; the faced fixture is the one that was missing.

Any new detail-page affordance is added to the same set.
