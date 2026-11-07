---
id: TKT-P8H6XPW
type: ticket
title: Help texts for every form placement
kind: enhancement
priority: medium
effort: m
status: ready
---

## Description

OpenVWR (Filament) shows help in more places than rela forms could. The OpenVWR
v2 port needs every placement to carry its help over.

Added to `data-entry.yaml` forms:

- field and relation `hint`: Markdown behind an info button next to the label
(Filament `hintIcon` tooltips and `hintAction` info modals).
- relation `help`: the SPA already read it, but the Go config dropped the key.
- step `details`: longer Markdown help, collapsed below the step description
(OpenVWR's collapsible information block per wizard step).
- form `description` now renders below the form heading. It was documented
but never shown.
- field/relation `help`, step `description` and form `description` render as
sanitized Markdown, so a help text can carry a link.

Relation cards now fall back to the relation type's description, as the picker
already did.
