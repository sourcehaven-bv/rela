---
id: RR-9HAD39K
type: review-response
title: FLIP positions go stale after a reflow
finding: After a resize that reflows the grid, the next data change slid every card from its old place.
severity: minor
resolution: 'A ResizeObserver on the board marks the positions stale when the width changes; the next render only measures. Height is ignored since it changes with every card. Test: ''slides nothing on the first render after the board width changed''.'
status: addressed
---
