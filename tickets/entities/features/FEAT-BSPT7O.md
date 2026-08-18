---
id: FEAT-BSPT7O
type: feature
title: Dashboard scales with graph size
summary: The dashboard's transferred payload and render work track what it displays, not the size of the graph behind it.
description: 'Every dashboard card fetches full entity records regardless of what it displays. The dogfooded tickets/data-entry.yaml has 7 cards, all display: count or display: breakdown -- displays needing only an aggregate -- yet one dashboard load transfers 4.1MB of JSON at 8000 entities. Measured through handleV1Search: 500 entities -> 256KB, 1000 -> 513KB, 2000 -> 1.0MB, 4000 -> 2.1MB, 8000 -> 4.1MB (clean linear growth). A single count card rendering one integer ships 379KB at 4000 entities. Covers making both the wire payload and the client-side render work proportional to what is actually shown.'
priority: medium
status: proposed
---
