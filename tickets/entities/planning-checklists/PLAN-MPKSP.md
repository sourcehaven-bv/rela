---
id: PLAN-MPKSP
type: planning-checklist
title: 'Planning: Relocate .rela/ user-local state to user config directory (cross-platform)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope (UPDATED after design review):**

**IN scope:**

1. New `internal/userstate` package — cross-platform per-user/per-repo
state service. Linux / macOS / Windows.
2. Storage: `os.UserConfigDir()/rela/repos/<repo-id>/`, scoped by a
single canonical fingerprint (`.rela/repo-id`, see §Fingerprint).
3. Unify existing XDG-ish paths onto the new service, injected at
the factory layer:
   - Delete `xdgStateHome()` + `runtime.GOOS` switch from
`internal/encryption/localstate.go`.
   - Drop the `~/.config/rela/key` tier from
`internal/encryption/loader.go:resolveIdentityPath`.
   - Drop the legacy `<root>/.rela/key` tier (see RR-H25H0).
4. Relocate out of `.rela/`:
   - `key` (age identity)
   - `documents/` (rendered-entity HTML cache)
   - `ui-state.json`, `user-defaults.yaml`, `palette.yaml`
   - `last_seen_version`
   - **`scheduler-state.json`** (moved per RR-Z1AUY)
5. Keep in `.rela/`:
   - `cache.json` (graph cache, rebuildable)
   - `fsstore-index.json` (tied to project tree mtimes)
   - `repo-id` (NEW file — the canonical fingerprint)
   - `ai.yaml` (team config)
   - `secrets.yaml` (script creds; separable concern)
   - `encryption.yaml` (marker)
6. Per-repo fingerprint: **single source — `.rela/repo-id`.** See
§Fingerprint.
7. Inter-process locking for security-critical state via
`github.com/rogpeppe/go-internal/lockedfile` (see RR-CDQ8O).
8. Cross-platform test coverage **enforced in CI** (see RR-VLKV5):
add macos-latest and windows-latest matrix entries for the userstate package
tests.

**OUT of scope:**

- Auto-migration from existing `.rela/key`, `.rela/ui-state.json` etc.
FEAT-KAJBD isn't in a tagged release. Users re-run `rela keys init` or manually
move files. Hard break — **no legacy read fallback**.
- `ai.yaml` / `secrets.yaml` relocation (separate concern).
- Pluggable non-FS backend.
- `rela config paths` or prune command (see RR-QNCMP, deferred).
- `project.Context` changes beyond a new accessor.

**Acceptance Criteria:**

1. **`internal/userstate` package** with narrow interface:
   ```go
   type Service interface { state.KV }
   type FSService interface {
       Service
       Root() string
       Path(key string) string
   }
   ```
`NewFS(projectRoot string) (FSService, error)` resolves `.rela/repo-id`
internally; cross-checks against `Keyring.RepoID()` when encrypted.
`NewForTest(root string) FSService` for tests.
- *Test:* service CRUD round-trip, fingerprint resolution,
encrypted/cleartext crossover.

2. **Identity precedence**: `$RELA_KEY_FILE` → `us.Path("key")`.
**No legacy `.rela/key` tier. No `~/.config/rela/key` tier.**
   - *Test:* `loader_test.go` cases for env, userstate, and
`ErrNoPrivateKey` when neither exists.

3. **`encryption.NewLocalState(svc)` takes the service.**
`xdgStateHome` deleted. `LoadVersion`/`StoreVersion` use `lockedfile` for
read-compare-write atomicity.
   - *Test:* existing `localstate_test.go` adjusted to use
`userstate.NewForTest`; new test for concurrent-writer correctness.

4. **Dataentry uses userstate** for ui-state, user-defaults,
palette, documents.
   - *Test:* `app_test.go` save→load via test service; verify files
land outside `paths.CacheDir`.

5. **Scheduler uses userstate** for `scheduler-state.json`.
`Workspace.State()` signature unchanged (still `state.KV`), but its root
changes.
   - *Test:* `scheduler_test.go` state persisted via injected
test service.

6. **Factory explicit construction**:
`NewFSFactory(fs, paths, us) (*FSFactory, error)`. Public struct-literal
construction removed from external contract. `UserState` must be non-nil.
   - *Test:* factory test exercises construction contract.

7. **`rela keys init` writes identity to `us.Path("key")`.**
Source-path warning when `--identity` argument is inside the project tree
(RR-242DF). `.gitignore` for `.rela/repo-id` written alongside the directory
creation.
   - *Test:* integration test confirms file location, mode 0o600,
and source-path warning.

8. **Cross-platform path resolution** in `userstate/paths.go`:
pure function `resolve(goos, env, userConfigDir)`. Table-driven test across
Linux/macOS/Windows. CI matrix runs userstate tests on macos-latest and
windows-latest.

9. **`$RELA_USER_STATE_DIR` validation** (RR-JEMLO):
   - Must be absolute; no NUL/control chars; directory on first write.
   - Reject paths inside project root.
   - `slog.Warn` + `out.WriteMessage` echo when resolved path
contains common sync substrings (`~/Dropbox`, `~/OneDrive`, `~/Library/Mobile
Documents`, `~/Library/CloudStorage`).

10. **`.rela/repo-id` git-tracking check** (RR-LDRW3):
    - On load, `git ls-files --error-unmatch .rela/repo-id`. If
tracked → error with instructions. Skip check if no `.git`.
    - File starts with `# DO NOT COMMIT — this file identifies your
per-machine user-state directory. Regenerate if leaked.`

11. **0o600 / 0o700 enforcement** (RR-HQGU7):
userstate's FS backend enforces strict perms uniformly. Doesn't reuse
`state.FSKV`'s 0o644 default.

12. **Platform indexer opt-out** (RR-628G7):
    - macOS: write `.metadata_never_index` at `<base>/rela/` on
first create.
    - Windows: set `FILE_ATTRIBUTE_NOT_CONTENT_INDEXED` on
`<base>\rela\`.
    - Best-effort; failure logs debug + continues.

13. **Error-string audit** (RR-5XEBB):
    - `app.ErrEncryptedRepoNeedsIdentity` text updated — no more
references to `.rela/key` or `~/.config/rela/key`.
    - `rela keys init` output updated to describe new location.
    - All references in `encryption/errors.go`, `docs/encryption.md`.

14. **No regression**: `just test`, `just lint`, coverage ratchet.

## Research

- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns
- [x] Looked for reference implementations
- [x] Reviewed rela concepts

**Existing Solutions:**

- `os.UserConfigDir()` stdlib — single cross-platform primitive.
`github.com/adrg/xdg` rejected (overkill). `github.com/kirsle/configdir`
rejected (thin wrapper).
- `github.com/rogpeppe/go-internal/lockedfile` — what Go's own
toolchain uses for cross-platform advisory file locks. Adopt for
security-critical state writes (see RR-CDQ8O).
- `github.com/google/uuid` — already a transitive dep via age.
Use for strict UUIDv4 validation of repo-id (RR-L3368).

**Similar patterns in codebase:**

- `internal/encryption/localstate.go` — existing XDG code to replace.
- `internal/encryption/loader.go:resolveIdentityPath` — hardcoded
tiers to replace.
- `internal/state/state.go:FSKV` — KV interface reused as
`state.KV`. New FS backend in userstate has stricter perms.
- `internal/storage/safefs.go` — existing atomic-write pattern.
- Reseal sentinel + last-seen-version are already per-machine,
per-repo keyed by `Keyring.RepoID()`.

**Reference implementations:** Go toolchain uses `os.UserConfigDir()`
+ `lockedfile`; age itself is agnostic about identity path.

## Approach

- [x] Technical approach chosen and documented
- [x] Builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:**

### Step 1 — `internal/userstate` package

`internal/userstate/service.go`:

```go
// Service holds per-user, per-repo state that must not be synced
// with the repo tree. It embeds state.KV so existing consumers
// keep their contract unchanged; the filesystem-specific operations
// live on FSService.
type Service interface { state.KV }

type FSService interface {
    Service
    Root() string
    Path(key string) string
}
```

`internal/userstate/fs.go`:

```go
type fsService struct {
    root string
    fs   storage.FS
}

// NewFS resolves the per-repo user-state directory for the project at
// projectRoot. It reads .rela/repo-id (generating it if absent for a
// cleartext repo). If the repo is encrypted it cross-checks .rela/repo-id
// against the Keyring.RepoID and errors on mismatch (catches a copied-in
// .rela/ from another project).
func NewFS(projectRoot string) (FSService, error) { ... }

// NewFSWithKeyring is the encrypted-repo variant: the caller has
// already loaded the keyring and passes its RepoID so the service
// can cross-check without re-loading.
func NewFSWithKeyring(projectRoot, keyringRepoID string) (FSService, error) { ... }

// NewForTest roots a service at an explicit directory. Tests and
// power-user overrides via $RELA_USER_STATE_DIR.
func NewForTest(root string) FSService { ... }
```

`internal/userstate/paths.go` — pure function:

```go
// resolve returns the base directory for rela user-state.
// Precedence:
//  1. $RELA_USER_STATE_DIR (absolute; validated for project-tree + sync-dir safety)
//  2. os.UserConfigDir()
func resolve(env func(string) string, userConfigDir func() (string, error)) (string, error)
```

`internal/userstate/lock.go` — thin wrapper over `lockedfile.Lock(path)` +
`.Unlock()`. Used by `StoreVersion` and reseal sentinel compound ops.
Non-security reads/writes skip locking (last-writer-wins is acceptable for
ui-state/palette/ scheduler-state on a single machine; multi-machine sync is not
the target use case).

`internal/userstate/platform_*.go` — per-platform indexer opt-out:

```go
// platform_darwin.go
func tagNotIndexed(baseDir string) error {
    return os.WriteFile(filepath.Join(baseDir, ".metadata_never_index"),
        nil, 0o600)
}

// platform_windows.go (GOOS=windows build tag)
func tagNotIndexed(baseDir string) error {
    // SetFileAttributesW(base, FILE_ATTRIBUTE_NOT_CONTENT_INDEXED)
    ...
}

// platform_other.go (GOOS=linux, bsd, etc.)
func tagNotIndexed(string) error { return nil }
```

### Step 2 — Replace `encryption/localstate.go`

- Delete `xdgStateHome()`.
- `NewLocalState(svc userstate.FSService)` takes the service.
- `LoadVersion` / `StoreVersion` use `svc.Get` / `svc.Put` on key
`last_seen_version`, wrapped in a
`lockedfile.Lock(svc.Path("last_seen_version.lock"))` for read-compare-write
correctness.

### Step 3 — Rewrite `encryption/loader.go:resolveIdentityPath`

Precedence becomes:

1. `$RELA_KEY_FILE` (unchanged; missing file is an error)
2. `us.Path("key")` if present
3. Else `ErrNoPrivateKey`

No legacy `.rela/key`, no `~/.config/rela/key`. Clean break.

`LoadFromDir(projectRoot string)` is replaced by
`LoadFromDirWithUserState(projectRoot string, us userstate.FSService)`.

### Step 4 — Factory explicit construction

```go
// NewFSFactory returns a factory wired with a user-state service.
// us must be non-nil.
func NewFSFactory(fs storage.FS, paths *project.Context,
    us userstate.FSService) (*FSFactory, error) {
    if us == nil {
        return nil, errors.New("app: NewFSFactory requires a user-state service")
    }
    return &FSFactory{fs: fs, paths: paths, us: us}, nil
}
```

`FSFactory` struct fields become unexported. Tests use
`userstate.NewForTest(t.TempDir())`.

`newCryptoFS` passes `f.us` to `encryption.NewLocalState`.

### Step 5 — Dataentry plumbing

`NewApp` gains `us userstate.FSService`. It constructs `state.NewFSKV` over
`us.Root()` — or better, `us` is itself a `state.KV`, so just pass it directly.

### Step 6 — Scheduler

`scheduler-state.json` moves to user-state. `Workspace.State()` signature stays
(`state.KV`); the underlying backend is the user-state service instead of
`state.NewFSKV(fs, paths.CacheDir)`.

### Step 7 — `rela keys init`

- Write age identity to `us.Path("key")` with 0o600 mode.
- Post-copy check for `--identity` source: if inside `projectRoot`,
warn (RR-242DF).
- On first repo-id creation, write `.gitignore` fragment if not
already present (`repo-id` to `.rela/.gitignore`).

### Step 8 — `internal/cli/flow.go`, `internal/cli/script.go`,
### `internal/encryption/reseal_sentinel.go`, `internal/desktop/`

All entry points that construct a factory or call `NewLocalState` get the
service wired. See §Files.

### Fingerprint — single source of truth

**Single fingerprint: `.rela/repo-id` UUIDv4.**

- On `rela init` (cleartext): write `.rela/repo-id` with a new UUIDv4.
- On `rela keys init` (encrypting an existing cleartext repo):
  - If `.rela/repo-id` already exists, write its value into
`recipients.age` as the `RepoID` field. Don't allocate a new one.
  - If `.rela/repo-id` is missing, use `Keyring.RepoID` (the one
`keys init` generates) and also write it to `.rela/repo-id`.
- On service open for an encrypted repo: cross-check
`.rela/repo-id` against `Keyring.RepoID()`. Mismatch → error
(`userstate.ErrRepoIDMismatch`) — signals a copied-in `.rela/` from another
project.
- Validate as strict UUIDv4 (RR-L3368).
- Include `# DO NOT COMMIT` header (RR-LDRW3).

The user-state dir root is **always** keyed by `.rela/repo-id`. Encryption state
(last_seen_version, reseal sentinel) uses the same dir — no divergence across
encrypt/decrypt transitions (RR-R3MKP).

**Files to modify / create:**

- NEW: `internal/userstate/service.go`, `fs.go`, `paths.go`,
`lock.go`, `platform_darwin.go`, `platform_windows.go`, `platform_other.go`,
plus `*_test.go`.
- NEW: `internal/project/repoid.go` — `ResolveRepoID(root, keyringRepoID string) (string, error)`
with git-tracked check, UUIDv4 validation.
- MODIFIED:
  - `internal/encryption/localstate.go` — take service, add locking.
  - `internal/encryption/localstate_test.go` — adjust setup.
  - `internal/encryption/loader.go` — drop legacy tiers.
  - `internal/encryption/loader_test.go` — updated cases.
  - `internal/encryption/reseal_sentinel.go` — take service.
  - `internal/encryption/reseal.go` — update diagnostic messages
(RR-TUUZA).
  - `internal/encryption/errors.go` — updated error messages.
  - `internal/app/factory.go` — explicit constructor, unexported fields.
  - `internal/app/factory_test.go` — cover contract.
  - `internal/dataentry/app.go` — consume service.
  - `internal/dataentry/app_test.go`.
  - `internal/dataentry/document.go` — no change if KV routed.
  - `internal/scheduler/scheduler.go` — state goes via service.
  - `internal/scheduler/scheduler_test.go`.
  - `internal/project/config.go` — expose `RepoID()` accessor.
  - `internal/cli/init.go` — generate `.rela/repo-id`, write
gitignore fragment.
  - `internal/cli/keys.go` — write identity to `us.Path("key")`,
source-path warning, drop `ensureKeyGitignored` for user path.
  - `internal/cli/root.go` — build `userstate.FSService`,
thread to factory.
  - `internal/cli/script.go`, `internal/cli/flow.go` — thread
service to lua runtimes (RR-DKYZN).
  - `internal/mcp/server.go` — thread service.
  - `internal/desktop/*.go` (Wails startup) — thread service.
  - `internal/workspace/workspace.go` — `State()` now returns the
user-state service instead of building a new `FSKV` over `paths.CacheDir`.
  - `cmd/rela/main.go`, `cmd/rela-server/main.go`,
`cmd/rela-desktop/main.go` — entry-point wiring.
  - `.github/workflows/ci.yml` — cross-platform matrix for
userstate package (RR-VLKV5).
  - `docs/encryption.md` — C2 section rewrite; key location;
`$RELA_USER_STATE_DIR` docs; repo-id explanation.
  - `CLAUDE.md` — user-state section; update Project Files table.

## Security Considerations

- [x] Input sources identified
- [x] Allowlist validation defined
- [x] Sensitive operations identified
- [x] Error messages don't leak secrets

**Input Sources & Validation:**

- `$RELA_USER_STATE_DIR`: must be absolute, no NUL/control chars,
directory on first write. **Reject if inside projectRoot.** **Warn if under
common sync dirs** (Dropbox, OneDrive, iCloud).
- `$XDG_CONFIG_HOME` (via `os.UserConfigDir()`): honored only when
non-empty; stdlib does the trim. No custom handling.
- `repo-id`: strict UUIDv4 validation (RR-L3368). Used as path
segment. Reject anything else.
- Keys passed to Get/Put: reuse `state.FSKV.validateKey`.
- `--identity` source path to `keys init`: check if inside
`projectRoot`, warn if so (RR-242DF).

**Security-Sensitive Operations:**

- Age identity write: file 0o600, dir 0o700. Windows ACL
limitation documented.
- `last_seen_version` read-compare-write: `lockedfile` advisory
lock. Protects rollback defense from concurrent writer degradation (RR-CDQ8O).
- Reseal sentinel compound ops: same locking.
- Atomic writes via `.tmp` + rename for crash safety.
- Platform indexer opt-out to avoid OS indexer leaks (RR-628G7).
- `.rela/repo-id` git-tracking check — prevents cross-collaborator
state-dir collision (RR-LDRW3).
- Error messages include paths (diagnostic); never include file
contents or key bytes. Existing `redactKey` discipline unchanged.

## Test Plan

- [x] Test scenarios per AC
- [x] Edge cases documented
- [x] Negative tests defined
- [x] Integration test approach

**Test Scenarios:**

- AC1 (package): `paths_test.go` table-driven across goos. `fs_test.go`
CRUD round-trip.
- AC2 (loader): tests for env / userstate / neither.
- AC3 (LocalState): concurrent-writer test — two goroutines racing
StoreVersion + LoadVersion, verify lockedfile prevents interleaved updates.
- AC4 (dataentry): round-trip via test service; verify files land
outside `paths.CacheDir`.
- AC5 (scheduler): state persists through service.
- AC6 (factory): `NewFSFactory(nil us)` → error.
Public struct-literal construction removed (compile-time check).
- AC7 (keys init): file lands at `us.Path("key")` with 0o600;
source-path inside projectRoot → warning captured.
- AC8 (paths): table-driven + CI matrix.
- AC9 ($RELA_USER_STATE_DIR): project-tree rejection,
sync-dir warning.
- AC10 (git-tracked repo-id): integration test using a temp git
repo; commit the file; verify load errors.
- AC11 (perms): file mode check after Put.
- AC12 (indexer opt-out): verify `.metadata_never_index` on
darwin; skipped on linux.
- AC13 (error strings): grep test that `.rela/key` does not
appear in user-facing error text.

**Edge Cases:**

- `$RELA_USER_STATE_DIR` absolute but inside projectRoot → reject.
- `$RELA_USER_STATE_DIR` under `~/Dropbox` → warn + continue.
- `.rela/repo-id` missing → generate.
- `.rela/repo-id` exists but not UUIDv4 → refuse (don't regenerate).
- `.rela/repo-id` exists AND is git-tracked → refuse.
- Encrypted repo, `.rela/repo-id` missing → generate from
`Keyring.RepoID()`.
- Encrypted repo, `.rela/repo-id` exists but differs from
`Keyring.RepoID()` → `ErrRepoIDMismatch`.
- Concurrent `StoreVersion` from two processes → serialized by
`lockedfile`.
- macOS case-insensitive FS: `.metadata_never_index` placed at
`<base>/rela/` — case doesn't matter; noted.
- Windows MAX_PATH 260 → documented; `rela config paths`
follow-up surfaces resolved paths.
- NFS-mounted user home: `lockedfile` handles this via
`flock`/`fcntl` with documented caveats. Note in godoc.

**Negative Tests:**

- `repo-id` with `..`, `/`, or NUL → reject.
- Key containing `../` → reject (reuse `state.FSKV.validateKey`).
- `$RELA_USER_STATE_DIR` is a file → error with clear message.
- `$RELA_USER_STATE_DIR` is relative → error.
- Missing HOME + no `UserConfigDir()` fallback → error.
- `git-ls-files` returns tracked for `.rela/repo-id` → error.
- Non-UUID content in `repo-id` → error (no silent regen).

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks (see above)
- [x] Effort estimated

**Risks:**

- **Hard break on `.rela/key`** (RR-H25H0 decision): users upgrading
from pre-release encryption builds must re-init. *Mitigation*: release notes.
FEAT-KAJBD isn't tagged yet, so affected user set is ~zero.
- **`lockedfile` dependency on NFS/CIFS**: advisory locks may be
unreliable on old NFS implementations. *Mitigation*: documented; not a target
deployment.
- **Call-site churn** (workspace, CLI, server, desktop, MCP,
scheduler, script, flow): bounded by factory-layer injection. Each entry point
gets a `userstate.FSService` constructed once.
- **CI cost**: macOS + Windows matrix adds ~2 minute runtime.
*Mitigation*: matrix only runs `go test ./internal/userstate/...` on extra
platforms — not the full suite.
- **Fingerprint mismatch on copied repos** (new failure mode,
intentional): `ErrRepoIDMismatch` when someone copies `.rela/` between projects.
*Mitigation*: clear error message with remediation steps.

**Effort: m (medium).** ~20 files touched, ~6-8 new files. CI matrix change.
Non-trivial test migration but bounded.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/encryption.md` — C2 section becomes "relocated; here's
where your key lives now"; add repo-id explanation; `$RELA_USER_STATE_DIR` env
var.
- [x] `rela keys init` / `keys generate` output strings.
- [x] `CLAUDE.md` — User Defaults, Project Files tables; new
"User-local state" section; `$RELA_USER_STATE_DIR` env var; per-platform path
table.
- [x] Release notes — hard break on `.rela/key`; must re-init or
manually move identity.
- [x] Error strings in `app.ErrEncryptedRepoNeedsIdentity`,
`encryption/errors.go`.
- [x] README.md — no change.

## Design Review

- [x] `/design-review` run before implementation.
- [x] All critical/significant findings addressed in plan.

**Design Review Findings:**
- Critical: RR-R3MKP, RR-LDRW3, RR-H25H0, RR-CDQ8O, RR-JEMLO
- Significant: RR-HNN0C, RR-D4KC3, RR-DKYZN, RR-Z1AUY, RR-TUUZA,
RR-VLKV5, RR-98TZZ
- Minor: RR-628G7, RR-242DF, RR-FS0SZ, RR-HQGU7, RR-5XEBB
- Nit: RR-L3368
- Deferred: RR-QNCMP (orphaned-dir GC — follow-up ticket)

All critical/significant findings are addressed above.
