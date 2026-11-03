---
id: TKT-H9GT3M
type: ticket
title: 'File channel store: inbox scan per principal'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

On the file channel store, one inbox computation walks every channel directory
for `ListReads` and `ListAddressed`. A live round with 20 principals and 1000
channels takes about 3 s (TKT-9YRRM5 AC 6 benchmark), so the inbox grid is
widened to 4 s on that tier.

Keep a per-principal index (read markers and addressed rows by target), or scan
the channel tree once per round for all principals, so a round fits the 2 s
grid. Then use the 2 s grid on every tier.

Follows TKT-9YRRM5.
