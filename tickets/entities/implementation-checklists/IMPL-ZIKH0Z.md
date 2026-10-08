---
id: IMPL-ZIKH0Z
type: implementation-checklist
title: 'Implementation: Make sandbox read paths operator-configured (RELA_SANDBOX_READ_PATHS)'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Tests: `TestParseReadPaths`, `TestHostReadOnlyReachesEveryRunner`,
`TestApplyHostEnv`, `TestSetHostReadOnlyRejectsPathsThatUndoTheSandbox`,
`TestSystemReadOnlyPathsHoldOnlyBinariesAndLibraries` (Linux),
`TestCmdRunnerBindsOnlyOperatorPaths`, `TestParse_ScanSocketsRemoved` (list,
`[]`, bare key), `TestWarnIfNoSandboxReadPaths`. Integration: the clamd job
(`TestClamd*`, real clamd + bwrap) now sets its binds through `SetHostReadOnly`,
so it exercises the operator path end to end.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Existing helpers reused: `scanWarnMeta` / `captureWarn` (dataentry), `Parse`
with a base schema (metamodel). Path lists are table data; expected values are
the cleaned inputs.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

On the atlas host (Debian 13, bwrap, TeX Live, clamav-daemon), 2026-10-06, a
v26.10.2 build with this change's read-path mechanism (no migrations differ
between v26.10.2 and develop):

- AC5: `rela render --transform=pdf DOC-001@concept` fails with no read paths
(`xdvipdfmx:fatal: Unrecognized paper format: a4`), and succeeds with
`RELA_SANDBOX_READ_PATHS=/etc/paperspecs:/var/lib/texmf`. Removing either path
breaks it; removing `/etc/fonts`, `/etc/alternatives` and `/var/lib/fontconfig`
does not. `pdffonts` lists OpenSans, OpenSans-Bold and OpenSans-Italic embedded.
- AC6: `clamdscan --fdpass` and `--stream` succeed inside the same bwrap
invocation with `/var/run/clamav/clamd.ctl:/etc/clamav/clamd.conf`.
- qpdf and exiftool run with system directories only.

Local (macOS), 2026-10-07, on the final code:

- AC3: `rela-server --help` shows `--sandbox-read-paths` with its default
taken from `RELA_SANDBOX_READ_PATHS`.
- AC7: `RELA_SANDBOX_READ_PATHS=/:/nonexistent/x:relative rela list ticket`
warns `ignoring sandbox read path path=/ reason="binds the whole host"` and
`ignoring non-absolute sandbox read path path=relative`; the missing path is not
reported by the CLI. With `-q` no warning is printed.
- `go test ./...` passes except `cmd/rela-desktop`
`TestChromeStyle_TargetsShippedClasses`, which needs the built SPA (no frontend
build in this worktree; unrelated). golangci-lint: 0 issues on macOS and with
`GOOS=linux` for cmdexec and attachment.

Code and security review added `CheckProjectNotExposed` (NewApp and `rela
render`), symlink-aware refusal of `/run`, `/var/lib`, `/home`, `/root`, a
writable-dir check in `validateSpec`, and merge-key detection for
`scan_sockets`; covered by `TestCheckProjectNotExposed`,
`TestSetHostReadOnlyChecksSymlinkTargets`, `TestWrapRefusesBindOverWritableDir`
and `TestParse_ScanSocketsRemoved`. The atlas list
(`/var/run/clamav/clamd.ctl`, `/etc/clamav/clamd.conf`, `/etc/paperspecs`,
`/var/lib/texmf`) passes these checks, and its project (`/srv/atlas/...`) lies
outside every listed path.

The design-review changes (path refusal, startup warning, desktop wiring) are
covered by unit tests; they were not re-run on atlas, because they do not change
which paths bwrap binds for an accepted list.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`ApplyHostEnv` replaces the two env reads that the CLI and desktop would
otherwise duplicate; `EnvUnconfinedCommands` replaces the string literal in
rela-server. Rejected paths are warned, not silently dropped.
