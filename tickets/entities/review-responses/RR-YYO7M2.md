---
id: RR-YYO7M2
type: review-response
title: Chart max-height is a magic number
finding: calc(100vh - 12rem) ignores header text and the entity-tab embedding, pushing the horizontal scrollbar off-screen.
severity: significant
resolution: measure() sets --chart-max-h from the room below the chart's top (min 320px), on resize too.
status: addressed
---
