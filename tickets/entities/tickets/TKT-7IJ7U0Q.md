---
id: TKT-7IJ7U0Q
type: ticket
title: Collapsible cards sections on the entity page
kind: enhancement
priority: medium
status: backlog
description: Split from TKT-4Q2EIVZ; see body.
---

## Description

Let `display: cards` sections take `collapsible: true` and render each row in
`RlAccordion` with a plain-text summary. Property fields only; prose editing is
a separate ticket. Split from TKT-4Q2EIVZ after its design review.

Constraints (RR-NFQ2D2G):

- Mount a card's body on first open, not at page load.
- Open the card whose anchor matches the route hash, so the owned-item
redirect lands on an open card.
- Refuse `collapsible` on side-panel sections and on non-cards displays.
