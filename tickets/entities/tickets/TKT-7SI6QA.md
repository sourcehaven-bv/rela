---
id: TKT-7SI6QA
type: ticket
title: 'Batch ACL when: traversals on caldav, feeds, tracer and gantt'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

TKT-205V2N answers `related()` in ACL `when:` through a per-operation memo
primed per page on the data-entry list serialization and in `PolicyReader`
chunks. The remaining per-row callers still make one `MatchingIDs` per grant
traversal per row when a policy uses `related()`:

- caldav (`caldav_backend.go` HiddenProperties per event)
- feeds (`feed_provider.go`)
- the visibility tracer decorator (every trace node)
- gantt subtree verdicts (`ganttSubtreeVerdicts`)

Prime each with the page or node set, and pin each with a `storetest.Counting`
budget (same count at 10 and 50 rows). TKT-205V2N adds a baseline test recording
the current per-row cost.
