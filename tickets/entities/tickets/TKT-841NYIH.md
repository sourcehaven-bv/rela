---
id: TKT-841NYIH
type: ticket
title: Scale run measures before autoanalyze and flakes on list reads
kind: chore
priority: medium
status: backlog
---

## Description

Found while measuring TKT-M58EQ1W. In three of five local scale runs, list-tasks
and list-open-tasks at scale 5 took about 390-410 ms p50, and the kanban column
up to 680 ms. The other two runs, one with the same binaries, took about 9 ms.

The slow statement was the relations read `rel_type = $1 AND from_id = ANY(...)
AND to_id = ANY(...)`. The scale run measures right after `rela dev seed`, and
nothing runs ANALYZE, so the plan depends on whether autoanalyze of `relations`
has finished. On the kept schema, after autoanalyze, the same request takes
about 40 ms.

This makes the weekly run flaky for reasons unrelated to the code under test.

## Approach

- Run `ANALYZE` on the seeded schema before the server starts, in
`tools/scalerun/cmd/scalerun` (`runScale`), so every run measures with current
statistics.
- Then check whether the plan before ANALYZE is also what a real tenant sees
after a bulk import, and if so file that separately.
