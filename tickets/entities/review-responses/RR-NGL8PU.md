---
id: RR-NGL8PU
type: review-response
title: Context read with ID@face ignores source_world
finding: visibleReader reads a named face literally, so the viewed face would be evaluated instead of the source world's face.
severity: significant
resolution: 'Planned: keep only the bare id and read inside the source world after nextActionSourceWorld rebinds ctx. Test: viewing X@published with an editorial source evaluates the editorial face.'
status: addressed
---
