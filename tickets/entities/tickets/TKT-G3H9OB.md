---
id: TKT-G3H9OB
type: ticket
title: Dashboard grouping and matrix axes over a related entity
kind: enhancement
priority: medium
status: backlog
---

## Description

Out of scope for the G3 dashboard work. Breakdown and matrix cards group on a
property of the source list's own type. Operators also want to group on a
property of a related entity (for example risks by the owning department of
their asset). That needs a related-entity axis that is gated the same way as the
rows: a hidden related entity must not contribute a group label or a count, and
the read must stay batched per page (collection-reads rule).
