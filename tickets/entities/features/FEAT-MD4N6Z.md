---
id: FEAT-MD4N6Z
type: feature
title: 'Operator customisation hooks: custom.css / custom.js against rela''s own SPA surfaces'
summary: In-place customisation of rela's own UI from the operator's project directory, with a three-tier stability contract.
description: 'Distinct from FEAT-BFDB9Q (custom apps), which are sandboxed iframes rendering as separate pages and cannot alter the main UI. This feature is the different-trust-model counterpart: an operator editing their own project directory already controls metamodel, Lua and ACL, so there is no privilege boundary left to defend, and customisation happens in place against rela''s own surfaces.'
priority: medium
status: proposed
---

## Motivation

Existing theming layers (palette.yaml, .relatheme packages, _rela.css tokens)
cover recolouring well but nothing beyond it. Custom apps (`apps/<id>/`) allow
arbitrary HTML/JS but only as a separate page in a sandboxed iframe — they
cannot alter the main UI.

The gap is customising rela's **own** surfaces in place. The iframe exists to
make *distributable* apps safe to install; an operator editing their own project
directory already controls the metamodel, Lua scripts and ACL. Different trust
model, different mechanism.

Explicitly an "if it breaks, you keep the pieces" feature. The palette/theme
system stays the supported path for ordinary branding; this is the escape hatch
for deployments that want something genuinely custom.

## Three tiers of contract

| Tier | Mechanism | Stability |
|------|-----------|-----------|
| 1 | Custom elements (`<rela-slot>`) | Real contract: tag name, attributes in, events out. A break is a rela bug. |
| 2 | `rela-` classes + `data-` attributes | Best-effort. Documented, but positional — structural changes may break skins. |
| 3 | Anything else (internal classes, scoped-CSS hashes, DOM structure) | No contract. |

Tier 3 is not a wart — it is what makes tier 1 affordable. Because there is an
escape hatch, we need not predict every extension point up front; we ship one
slot and let tier 3 absorb the rest. Customisations that recur in the wild are
the signal for what to promote into tier 1 later.

## Deliberately excluded

No `window.rela` JS API. Naming a function makes a promise, and this tier is
explicitly promise-free. Operators get the DOM, MutationObserver, CustomEvent,
and fetch against the REST API with the session cookie already attached.
