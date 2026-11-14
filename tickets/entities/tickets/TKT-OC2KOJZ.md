---
id: TKT-OC2KOJZ
type: ticket
title: Drop stale realtimelimits allowance for channels in arch-lint
kind: chore
priority: low
effort: xs
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

## Description

GitHub issue #2235. The `channels.MaxBatch` godoc says channels may not import
`realtimelimits`, and the archplan layer rule keeps it that way since 1d2f320.
`.go-arch-lint.yml` still listed `realtimelimits` in `channels.mayDependOn`, a
leftover from #2210. Remove the entry so the linter enforces the documented
rule.
