---
id: RR-3XUAR2
type: review-response
title: Nested links in a linked table column
finding: 'With cell.link the widget sat inside an <a>: nested anchors, and @click.stop blocked the SPA handler while native navigation still ran.'
severity: significant
resolution: A cell inside a link gets no entityId and renders plain names. Pinned by 'shows the name without a nested link in a linked column'.
status: addressed
---
