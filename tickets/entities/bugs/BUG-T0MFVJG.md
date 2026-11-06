---
id: BUG-T0MFVJG
type: bug
title: Renumber after an explicit order write rewrites other sources' edges unchecked on the data-entry path
description: An explicit incoming order write that collapses the gap renumbers other sources' edges without their relation meta grant or update ACL on the data-entry handle; a move checks both.
priority: medium
effort: m
status: backlog
---

## Description

An explicit order write (`PATCH .../relations/{rel}/{target}` with `meta:
{_order_in: n}`) that collapses the gap between two incoming edges makes
`Manager.runRenumberAfterUpdate` renumber every edge into the target. That
rewrites the order value of edges owned by other sources.

A position move on the same side checks the order key against the relation meta
grants of every source whose edge it may rewrite, in the data-entry handler and
(since BUG-FQZ5Q1L) in the field-gated manager. The renumber after an explicit
write is checked only on a field-gated manager handle. The data-entry handle is
not field-gated, and its explicit meta path checks only the written edge's
source, so on that path the renumber still rewrites other sources' edges
unchecked. It also does not run the per-source update ACL check that a move's
incoming densify runs (`readableOrderSiblings`), and it rewrites edges whose
source the caller cannot read.

Decide one rule for order-preserving rewrites of other sources' edges (engine
maintenance, or a write that needs each source's grant) and apply it to the
move, the renumber and both handles.

Found in review of BUG-FQZ5Q1L.
