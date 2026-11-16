---
id: RR-MMESVPT
type: review-response
title: PageDown during a row load skips the rest of the walk
finding: While the position of a new row loads, walk.position is undefined, so PageDown fell through to the next step.
severity: significant
resolution: 'PageView swallows PageDown and PageUp while the walk state is loading. Vitest case: holds PageDown while the position loads.'
status: addressed
---
