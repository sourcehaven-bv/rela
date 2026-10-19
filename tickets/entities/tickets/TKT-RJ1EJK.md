---
id: TKT-RJ1EJK
type: ticket
title: 'Fuzz sweep: install Wails Linux deps for the build gate'
kind: chore
priority: medium
effort: xs
completed: "2026-10-19"
status: done
---

## Description

The weekly fuzz sweep failed on 2026-10-19 (run 39210626821). The build gate in
`scripts/fuzz-all.sh` runs `go build ./...`, which includes `cmd/rela-desktop`.
That package links Wails v3, whose Linux build needs GTK4 and WebKitGTK headers.
The runner did not have them, so the build gate exited 2 before any target ran.

## Fix

Install `libgtk-4-dev` and `libwebkitgtk-6.0-dev` in `fuzz-sweep.yml`, as the
jobs in `ci.yml` already do.
