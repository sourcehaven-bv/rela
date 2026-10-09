---
id: TKT-0RJJ72
type: ticket
title: Version sweep selects only rows without a stored hash
kind: refactor
priority: low
effort: s
started: "2026-10-09"
completed: "2026-10-09"
status: done
description: 'Each sweep tick probed the latest version of every live row (about 185 ms at 50,000 entities; RR-N2MXHR). Version-table triggers now keep a stored content_hash equal to the latest version''s, so the sweep selects content_hash IS NULL through a partial index. Atlas: TASK-Y73Y9.'
---

## Description

Follow-up to BUG-1DWMYO (RR-N2MXHR). Each sweep tick probed the latest version
of every live row. A stronger invariant lets the sweep select only rows whose
stored hash is NULL: a non-NULL hash means the current lifecycle's latest
version has that hash. Triggers on the version tables and on live inserts clear
the hash when that may stop holding; the write-back checks the latest version.
Tracked in Atlas as TASK-Y73Y9.
