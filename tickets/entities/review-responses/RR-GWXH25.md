---
id: RR-GWXH25
type: review-response
title: Breakdown keyed by label; Unknown vs None
finding: Two values with the same label collide as v-for keys; unset reads Unknown in breakdown but None in ratio.
severity: nit
resolution: Breakdown rows key on the group value; the ratio uses Unknown too.
status: addressed
---
