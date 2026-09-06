---
id: TKT-9ZC55U
type: ticket
title: Migrate rela-desktop to Wails v3
kind: enhancement
priority: medium
effort: l
status: backlog
---

Move `cmd/rela-desktop` from Wails v2.15.0 to v3.0.0-beta.16, then add
multi-window, single-instance handoff, window state persistence and Finder
registration for `.rela` bundles.

## Two questions that had to be answered first

Both verified with a throwaway spike against the real tickets project:

- `AssetOptions.Handler` still takes a Go `http.Handler` — same shape as v2, so
`Desktop.ServeHTTP` ports unchanged.
- **SSE streams incrementally through it.** Load-bearing: `watcher.go handleSSE`
hard-fails 500 when the writer is not an `http.Flusher`, so live-reload would
have broken outright.

`go build -tags production` still works; no `wails3` CLI or Taskfile needed.

## Bugs this fixed

- No shutdown hook — the scheduler and services leaked on every quit.
- No single-instance lock — a second launch could not open a project the first
held and simply failed.
- Window geometry was hardcoded 1280x800 every launch.
- The icon rendered edge-to-edge, so macOS inset it into its own grid.

## Findings worth keeping

- **Windows must be created on the main thread**, or the window is registered
but never realised: no window appears and no error is returned. `InvokeAsync`,
not `InvokeSync` — the latter deadlocks if called from a menu callback.
- **v3 does not inject its runtime.** It serves `/wails/runtime.js` but the page
must request it; without that `window.wails` is undefined and every bound call
fails.
- **`Menu.Set` panics before the native menu exists** — documented as a silent
no-op, but it is a nil interface conversion.
- Two tests initially passed *with the bug re-introduced* because they matched
prose in a comment rather than code. Both rewritten and mutation-checked.

## Not verified

The Windows build — no host available. The Linux desktop release target was
dropped; v3 requires GTK4/WebKitGTK 6.0 and the dock service is a stub there.
