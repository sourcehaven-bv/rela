---
id: RR-ZUX3FW
type: review-response
title: Card index can point at another card after a config reload
finding: The SPA addresses a card by its config index. A reload between /_dashboard and the card fetch can shift indexes and serve one card's numbers under another's tile.
severity: significant
resolution: /_dashboard serves each card's config index; the card response echoes the card title and display and the SPA discards a response whose title or display differs from the tile it asked for.
status: addressed
---
