---
id: TKT-9G39MMA
type: ticket
title: Rename dataentrywire remotes adapter to remotes_wiring.go
kind: chore
priority: low
effort: xs
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

## Description

`internal/dataentrywire/remotes.go` adapts the scheduler to the
`dataentry.RemoteService` port, so it is wiring. Rename it and its test to
`remotes_wiring.go` and `remotes_wiring_test.go` so the architecture profile
reads the file as wiring. No code changes.
