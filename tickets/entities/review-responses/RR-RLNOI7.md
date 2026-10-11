---
id: RR-RLNOI7
type: review-response
title: Hidden mount does not re-centre
finding: If mounted at clientWidth 0, today lands at the left edge once shown.
severity: minor
reason: Gantt tabs mount with v-if, so the chart is measured when shown; revisit if tabs keep hidden panes mounted.
status: deferred
---
