---
id: TKT-EBTVUL8
type: ticket
title: Gantt root view reads every hierarchy row to roll up dates
kind: enhancement
priority: low
status: backlog
---

## Description

After TKT-K7EM7Z5 the gantt root view on postgres grows linearly with the data:
p50 165 ms at scale 1 (19k entities) and 880 ms at scale 5 (96k). Half of the
time is reading every source entity's header, with all of its properties, so the
roll-up can fold the dates of every descendant. The response itself carries at
most `max_nodes` rows.

To grow with the rows shown, the roll-up has to run in the store: a recursive
aggregate over the gated hierarchy, with the same ACL row gate, `visible:` date
redaction, `where:` filters, `multi_parent: first` choice and cycle policies as
the Go fold. Where that cannot be matched exactly the request falls back to the
full build, as the drill fast path already does. A smaller step is a header read
that projects only the properties the chart uses.

`tools/scalerun/scalerun.yaml` holds the gantt at `ratio_max: 8`; lower it to
the default when this lands.
