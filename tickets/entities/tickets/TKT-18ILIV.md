---
id: TKT-18ILIV
type: ticket
title: Saved (materialized) roll-ups
kind: enhancement
priority: low
effort: l
status: backlog
---

## Description

Follow-up of TKT-6I2OPG. Roll-ups are computed at read time with the reader's
gate and never stored. A saved variant (for sorting, search and export at scale)
must define whose access computes the stored value and how it is invalidated
when related entities or edges change.
