---
id: TKT-MAA5R1
type: ticket
title: Make filesystem migration stale-break acquisition atomic
kind: chore
priority: medium
effort: xs
tags: regression
status: done
---

The concurrent stale-break regression test is flaky in CI: two fsLock contenders
can both report successful acquisition. Keep the lock-break marker for the
complete stale replacement so removal and ownership publication form one
critical section.
