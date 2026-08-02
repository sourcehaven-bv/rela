---
id: TKT-PZTP1L
type: ticket
title: Remove dead UIState persistence (userStateStore, .rela/ui-state.json)
kind: chore
priority: low
effort: xs
status: done
---

Follow-up flagged on [[TKT-I37338]]: deleting the server-rendered nav path
removed `navElements`, the LAST production consumer of
`userStateStore.loadUIState`. The `.rela/ui-state.json` sidebar-group collapse
persistence was a server-rendered-UI mechanism; the SPA has no per-group
collapse at all (only a session-local whole-sidebar toggle in the Pinia ui
store) and never calls any UIState endpoint — none exists.

## What

Deleted the whole dead layer:

- `internal/dataentry/userstate.go` (userStateStore + loadUIState/saveUIState —
its entire remaining surface; logo/palette/defaults moved to their own services
long ago)
- `App.userState` field + wiring (NewApp and rebindApp), `uiStateFile` const
- `UIState` alias (dataentry/config.go) and `dataentryconfig.UIState` type
- `TestUIStateLoadSave`

**Kept**: the `collapsed` field on navigation-group config and on the
`/_sidebar` wire response (config/API compatibility). `docs/data-entry.md`
corrected: it claimed collapse state "is persisted server-side in
`.rela/ui-state.json`" — now documents that the current SPA renders groups
always expanded and the persistence mechanism is gone.

## Verification

- `go test -race ./internal/dataentry/ ./internal/dataentryconfig/` green
- `golangci-lint` 0 issues; `just plimsoll`; default/`postgres`/`memorybackend` builds
