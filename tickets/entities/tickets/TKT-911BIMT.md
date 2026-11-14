---
id: TKT-911BIMT
type: ticket
title: Page reader accepts only gated Related entries
kind: chore
priority: low
effort: s
status: backlog
---

## Description

`visibility.PageQuery.Related` (TKT-Z099FGY) is ANDed onto the caller's read
scope without the reader checking that each entry passed the caller's traversal
gate. The field only narrows, and its one producer
(`appbuild.QueryConditions.Lower`) gates every entry, so there is no current
exposure. A future caller that builds an ungated entry would let the row set
depend on edges the caller cannot see.

Make the contract a type: the reader accepts only a gated-relation value that
the traversal gate produces, or the reader gates the traversal specs itself.
Deferred from review response RR-CY6WIFE.
