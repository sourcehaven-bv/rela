---
id: TKT-NA2FG3A
type: ticket
title: 'rela test: scheduler ticks under the fake clock (t.advance runs schedules)'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`t.advance("3d")` should run every schedule that falls due in that period, at
the fake time, and drain the jobs it enqueues. Needs an exported scheduler entry
point (`Tick(ctx, now)` or `RunTask(ctx, name)`); today only `Run`, `UseQueue`
and `WatchConfig` are exported.
