---
id: RR-W3F0I47
type: review-response
title: Toned cells shift card alignment
finding: A toned cell had more padding and a label row, so its cards did not line up with plain neighbours.
severity: minor
resolution: The label is positioned in the padding row. A board with any toned cell gives every cell the toned padding (rl-swimlane-board--toned). The story now mixes toned and plain cells.
status: addressed
---
