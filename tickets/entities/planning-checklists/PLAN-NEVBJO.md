---
id: PLAN-NEVBJO
type: planning-checklist
title: 'Planning: Make sandbox read paths operator-configured (RELA_SANDBOX_READ_PATHS)'
started: "2026-10-07"
completed: "2026-10-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:

1. Reduce the built-in sandbox read list (`internal/cmdexec/sandbox_linux.go`)
to the binary and library directories: `/usr`, `/bin`, `/sbin`, `/lib`,
`/lib64`, `/lib32`.
2. Add a host-level operator list: `RELA_SANDBOX_READ_PATHS` (PATH-style),
applied by all three composition roots: `rela-server` via `--sandbox-read-paths`
(defaults to the env var), the CLI and rela-desktop via
`cmdexec.ApplyHostEnv()`, which also applies `RELA_UNCONFINED_COMMANDS` (the
desktop never applied that before). 2a. Refuse paths that would undo the
sandbox: `/`, `/etc`, `/proc`, `/dev`, `/tmp` and parents of the temp dir. 2b.
Warn at startup (Linux) when a scan or transform is configured and the list is
empty.
3. Remove `cmdexec.DefaultScannerSockets` and `cmdexec.DefaultScannerConfigs`
and the attachment runner's use of them.
4. Remove `attachments.scan_sockets`; a schema that still sets it fails
validation with a message naming the replacement.
5. Docs: `docs/transforms.md` (new "Sandbox read paths" section, verified
Debian 13 list for pandoc+xelatex), the attachment-security guide (unit sets the
variable, clamd section rewritten, upgrade note).

OUT:

- Per-runner lists (e.g. a scan-only variable). Decided with the user: one
host-wide list. Revisit only if a path needed by one tool is sensitive for
another.
- A deprecation period for `scan_sockets`. Decided with the user: remove in the
same release that drops the built-in paths, so operators change their setup
once.
- macOS read confinement. Still not possible with `sandbox-exec`; unchanged.
- A host config file format. Rejected in favour of the existing env + flag
pattern (`RELA_UNCONFINED_COMMANDS`, `RELA_DATABASE_URL`).

**Acceptance Criteria:**

1. **No paths beyond the system directories unless the operator lists them.**
Test: `TestHostReadOnlyReachesEveryRunner` (reset case) and
`TestCmdRunnerBindsOnlyOperatorPaths` (unset case) assert an empty extra list;
`Describe()` reports "reads: system directories only".
2. **The operator list reaches every runner.** Test:
`TestHostReadOnlyReachesEveryRunner` (cmdexec.New, the constructor every runner
uses) and `TestCmdRunnerBindsOnlyOperatorPaths` (attachment runner).
3. **Server, CLI and desktop read the same setting.** Test:
`TestApplyHostEnv` (CLI + desktop wiring); `rela-server --help` shows
`--sandbox-read-paths` defaulting to `$RELA_SANDBOX_READ_PATHS`; `rela render`
on atlas honours the env var (AC5).
4. **`scan_sockets` fails loudly.** Test: `TestParse_ScanSocketsRemoved`: a
schema with the key fails to parse and the error names
`RELA_SANDBOX_READ_PATHS`; the same schema without it parses.
5. **PDF export works on Debian 13 with the documented list.** Test (manual, on
the atlas host): patched v26.10.2 `rela render --transform=pdf DOC-001@concept`
fails with no read paths and succeeds with `/etc/paperspecs:/var/lib/texmf`;
`pdffonts` shows Open Sans embedded.
6. **ClamAV scanning works with the operator list.** Test: `TestClamd*` in CI
(clamd integration job) now sets the Debian socket + `clamd.conf` through
`SetHostReadOnly`; manual: `clamdscan --stream` and `--fdpass` succeed in the
same bwrap invocation on atlas with those two paths.
7. **Non-absolute and sandbox-undoing entries are dropped with a warning.**
Test: `TestHostReadOnlyReachesEveryRunner` (`relative/x`),
`TestWithExtraReadOnlySkipsNonAbsolute`,
`TestSetHostReadOnlyRejectsPathsThatUndoTheSandbox`.
8. **An upgraded host with commands but no read paths is told at startup.**
Test: `TestWarnIfNoSandboxReadPaths`.
9. **The built-in list stays binaries and libraries only.** Test:
`TestSystemReadOnlyPathsHoldOnlyBinariesAndLibraries` (Linux).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change; the design options were laid out to the user in chat on 2026-10-06 and decided there)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (see above).

**Existing Solutions:**

- No library: the mechanism (bwrap `--ro-bind-try`) already exists in
`internal/cmdexec`; only the source of the path list changes.
- In-repo pattern reused: `cmdexec.SetUnconfinedByDefault` +
`RELA_UNCONFINED_COMMANDS` (`internal/cmdexec/sandbox_options.go`,
`cmd/rela-server/main.go`, `internal/cli/kong.go`): a host-level knob set once
by both composition roots before any runner is built. The new `SetHostReadOnly`
mirrors it exactly.
- `WithExtraReadOnly` keeps its validation (absolute paths only) and is reused
for the per-runner path.
- Reference: Flatpak (also bwrap-based) grants filesystem access per app via
operator/manifest-declared `--filesystem=` entries rather than a compiled-in
list; same principle.
- Prior tickets: TKT-ZP1EE3 added `DefaultScannerConfigs` to fix exactly this
class of problem for clamd (`/etc` excluded, `clamd.conf` needed). This ticket
is the general form of that fix.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `cmdexec.systemReadOnlyPaths` replaces `readOnlyPaths`, with only the
system directories.
- `cmdexec.SetHostReadOnly(paths) []string`: validates (absolute only, warns
and skips otherwise; warns but keeps a path that does not exist yet, since a
daemon socket may appear later), cleans, stores in an `atomic.Pointer`.
`cmdexec.New` seeds `Runner.extraReadOnly` from it, so every runner (export
engine, document command, attachment runner) gets the list without plumbing.
- `cmdexec.ParseReadPaths` splits with `filepath.SplitList`, drops empties.
- `Describe()` includes the read paths so the startup log shows the posture.
- `attachment.NewCmdRunner` loses its options and built-in binds.
- `metamodel.AttachmentsConfig.ScanSockets` becomes `RemovedScanSockets`
(same yaml tag) and `validateAttachments` turns its presence into a schema
validation error. Needed because the loader does not reject unknown nested keys;
deleting the field would silently drop the operator's paths.
- `cmd/rela-server`: `--sandbox-read-paths` (default `$RELA_SANDBOX_READ_PATHS`)
applied in `applyCommandConfinement`, which warns about listed paths that do not
exist; `Describe()` reports the list in effect.
- `cmdexec.ApplyHostEnv()` applies both env settings; `internal/cli/kong.go`
calls it after logging is configured, `cmd/rela-desktop` after
`configureLogging` and before any project loads.
- `unsafeReadPath` rejects `/`, `/etc`, paths at/under/above `/proc`, `/dev`,
and `/tmp` / the temp dir or their parents, in both `SetHostReadOnly` and
`WithExtraReadOnly`.
- `warnIfNoSandboxReadPaths` (dataentry) warns at startup.

**Alternatives rejected:**

- Add `/etc/paperspecs` to the built-in list: fixes one host, leaves the next
missing path for the next release. Rejected by the user.
- Host config file (`/etc/rela/sandbox.yaml`): allows per-path comments, but
adds a format, a lookup path and precedence rules; the env pattern already
serves the other host-level settings.
- Keep built-in defaults and let the env var extend them: keeps the
distro-specific list in rela, which is the problem.
- Keep `scan_sockets`: two places for the same kind of setting, one of them in
project content that should not decide what the host exposes.

**Files to modify:**

- `internal/cmdexec/sandbox_linux.go`, `sandbox_options.go`, `cmdexec.go`,
`sandbox_darwin.go` (comment), `sandbox_test.go`
- `internal/attachment/cmdrunner.go`, `cmdrunner_test.go`,
`clamd_integration_test.go`
- `internal/metamodel/attachments.go`, `attachments_test.go`, `loader.go`
- `internal/dataentry/app.go`
- `cmd/rela-server/main.go`, `internal/cli/kong.go`, `cmd/rela-desktop/main.go`
- `internal/dataentry/handlers_attachment.go` (+ scanwarn test)
- `docs/transforms.md`, `docs-project/entities/guides/GUIDE-attachment-security.md`
(+ regenerated `docs/attachment-security.md`), `scripts/clamav-vm-test.sh`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `RELA_SANDBOX_READ_PATHS` / `--sandbox-read-paths`: host operator only
(process environment, unit file, command line). Validation: absolute paths only;
others are warned and skipped. No content-derived input reaches it.
- `attachments.scan_sockets` (project schema): no longer honoured; rejected at
load. This removes the one way project content could widen what the host exposes
to a converter.

**Security-Sensitive Operations:**

- The read list IS the control against local file disclosure through
converters (`\input{/etc/passwd}` in an export). After this change the default
exposure is smaller (no `/etc/fonts`, `/etc/alternatives`, clamd paths), and
every addition is an explicit operator decision. Paths that would undo the
sandbox (`/`, `/etc`, `/proc`, `/dev`, the temp dir and its parents) are
refused. Docs and the flag help state that every listed path is readable, and
its unix sockets connectable, by every command; list single files and socket
files, never `/run`.
- Warnings name the offending path, which is operator-supplied configuration;
nothing secret is logged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the "Test" line under each acceptance criterion.

**Edge Cases:**

- Empty variable, doubled or trailing separator: `ParseReadPaths` drops empty
entries (`TestParseReadPaths`).
- Trailing slash (`/etc/paperspecs/`): cleaned to `/etc/paperspecs`.
- Path that does not exist: kept, skipped by bwrap's `-try`; rela-server warns
at startup, the CLI stays quiet.
- Path under `/tmp`: bound after the `--tmpfs /tmp` mount (existing ordering,
pinned by `TestSandboxWritableDirUnderTmp` and the socket-bind test).
- Path containing `:`: not expressible; documented limitation of the PATH
format.
- `SetHostReadOnly` called before any runner (both composition roots do so at
startup); runners copy the list at construction.

**Negative Tests:**

- Relative entry → dropped (`TestHostReadOnlyReachesEveryRunner`).
- Schema with `scan_sockets` → load error naming the replacement
(`TestParse_ScanSocketsRemoved`).
- No read paths on Debian 13 → PDF export fails (manual, AC5).

**Integration:** clamd integration job (`TestClamd*`, real clamd + bwrap in CI);
`scripts/clamav-vm-test.sh` runs the guide's unit, which now sets the variable;
manual end-to-end render on atlas.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Breaking upgrade.** A host relying on the built-in paths loses them: scans
fail closed (uploads rejected) and some exports fail. Mitigations: startup log
states the paths in effect or "system directories only"; upgrade note in the
attachment-security guide; `scan_sockets` fails loudly instead of silently; the
sourcehaven atlas host gets the variable via Ansible before the rela bump
(devops `fix/atlas-sandbox-read-paths`).
- **One list for all commands.** pandoc can now read the clamd socket and
`clamd.conf` on hosts that list them. `clamd.conf` is world-readable and holds
no credential; the socket only allows scanning. Documented.
- **Global state.** `SetHostReadOnly` is package-level, like
`SetUnconfinedByDefault`; tests reset it in `t.Cleanup`.

**Effort:** m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/transforms.md`: new "Sandbox read paths" section; reads row in the
controls table.
- [x] `docs-project/entities/guides/GUIDE-attachment-security.md` (generates
`docs/attachment-security.md`): unit sets the variable; clamd section; upgrade
note.
- [x] `rela-server --help`: new flag text.
- [x] ~~docs/metamodel.md~~ (N/A: `scan_sockets` was documented only in the attachment-security guide, which is updated)
- [x] ~~CLAUDE.md~~ (N/A: the cmdexec rules there still hold; no new convention)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings** (2026-10-07, cranky-code-reviewer; all addressed,
details in each review-response):

- RR-19CVEJ (critical): rela-desktop never applied the read paths → `ApplyHostEnv` in all env-based roots.
- RR-INX7D4 (significant): operator paths could shadow the sandbox's mounts → `unsafeReadPath`.
- RR-U42OLN (significant): listed directories expose sockets for connect → documented; refuse `/`.
- RR-SY1B7G (significant): upgrade path clamd-only, failure silent → upgrade note + startup warning.
- RR-C5ALAE (minor): `scan_sockets: []` accepted → key-presence check.
- RR-FUW561 (minor): CLI warned before logging setup → reordered, no missing-path warning in CLI.
- RR-RYKBHZ (minor): every converter-running process needs the variable → documented; atlas uses the shared EnvironmentFile.
- RR-1YJ0TC (minor): wiring/invariant tests → four tests added; CI xelatex job not added (manual AC5).
- RR-XHP5OF (nit): list logged twice → server only warns on missing paths.
