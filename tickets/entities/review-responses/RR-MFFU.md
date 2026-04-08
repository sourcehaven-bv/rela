---
id: RR-MFFU
type: review-response
title: Manually-synced goldens between Go testdata and frontend __fixtures__ are a guaranteed drift source
finding: |-
    internal/dataentryconfig/palette_parity_test.go writes goldens to `internal/dataentryconfig/testdata/generate_dark_goldens.json` and frontend/src/utils/palette.test.ts reads from `frontend/src/utils/__fixtures__/generate_dark_goldens.json`. The parity test comment (line 16-21) tells the developer to manually copy the file. This is the single thing that 'manually keep these in sync' code review patterns always fail at — the next person to tweak the dark generation algorithm will run `UPDATE_GOLDENS=1`, see green Go tests, push, and the TS port will silently drift until someone runs the frontend suite.

    Fixes (cheapest first):
    1. **Symlink** `frontend/src/utils/__fixtures__/generate_dark_goldens.json` → `../../../../internal/dataentryconfig/testdata/generate_dark_goldens.json`. One file, one source of truth, zero copy. Vitest reads it via the resolved path. Add a comment at the top of the parity test explaining the symlink.
    2. **CI check**: a one-liner `diff` between the two paths in `just ci` (or a pre-commit hook) — fail loudly if they drift.
    3. **Generate-time copy**: have the Go test, when run with `UPDATE_GOLDENS=1`, also write to the frontend path.

    (1) is what I'd actually do. (3) is the second-best because it makes the workflow self-healing.

    Without one of these the ticket's parity guarantee is provisional at best.
severity: significant
resolution: Eliminated the manual copy. The TS test (`frontend/src/utils/palette.test.ts`) now imports the goldens directly from `../../../internal/dataentryconfig/testdata/generate_dark_goldens.json` (the Go-generated single source of truth). Deleted the duplicate `frontend/src/utils/__fixtures__/` directory. The Go test still owns regeneration via `UPDATE_GOLDENS=1`. Vitest tests pass; the relative cross-package import is type-checked and bundled-out-by-default since it's only used in test files.
status: addressed
---
