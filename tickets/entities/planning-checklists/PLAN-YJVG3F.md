---
id: PLAN-YJVG3F
type: planning-checklist
title: 'Planning: Secrets hygiene: enforce 0600 on .rela/secrets.yaml and support systemd LoadCredentialEncrypted'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
1. Warn (not refuse) when `.rela/secrets.yaml` is group- or world-readable.
2. Read `$CREDENTIALS_DIRECTORY` (systemd `LoadCredentialEncrypted=`) as a
secrets source, **scoped per project** (see RR-Y2O7C6).
3. Document both in `docs/lua-scripting.md` and `docs/mail.md`.

OUT:
- Encrypting `secrets.yaml` at rest (sops/age) — rejected on the ticket.
- macOS Keychain backend.
- Refusing to load a permissive file (warn only; failing closed would break
working deployments on upgrade).
- Changing `.rela/` from 0755 — it holds cache/index files that are not
secret, and tightening it is a separate blast radius.

**Acceptance Criteria:**

1. A `secrets.yaml` with mode 0644 logs exactly one `slog.Warn` naming the path
and the mode; `Load` still returns the secrets.
2. A `secrets.yaml` with mode 0600 logs nothing.
3. With `CREDENTIALS_DIRECTORY` set and a matching credential file inside,
`Load` reads from there and ignores the project file.
4. With `CREDENTIALS_DIRECTORY` set but no matching file inside, `Load` falls
back to the project file (systemd sets the var for any unit using
`LoadCredential*`, including units loading unrelated credentials).
5. The permission warning is skipped for the credentials directory (systemd
owns those modes; 0400 is already correct and not ours to police).
6. Repeated `Load` calls against the same permissive path warn **once**, not
once per call (RR-V1G22E).
7. Two different projects in one process with one `CREDENTIALS_DIRECTORY` do
**not** receive each other's secrets (RR-Y2O7C6).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small change; the investigation is recorded on the
ticket body.

**Existing Solutions:**

- **sops** (`github.com/getsops/sops/v3`; the `go.mozilla.org/sops` path is dead
— the module declares the getsops path and Go refuses the old one). Rejected:
links all five KMS backends unconditionally (139 modules, ~58 MB, roughly
doubling the binary), and the ENV-key deployment model is not an improvement.
- **filippo.io/age** (5 modules, +1.2 MB). Rejected for *this* ticket: whole-file
encryption whose key still has to live somewhere local. Already adopted for repo
data encryption — see DEC-D5P4X / [[repo-encryption]], whose threat model
explicitly names "subprocess inheritance of `$RELA_KEY_FILE`" as a reason to
prefer a key file over an env var. Same reasoning drives this ticket.
- **systemd credentials** (`LoadCredentialEncrypted=`): per-service tmpfs,
mode 0400, service-user-owned, NOT inherited by child processes, keys optionally
TPM2-backed. No dependency — it is an env var and a directory.
- Codebase prior art: `slog.Warn` for degraded-but-working config is the
established pattern (`internal/appbuild/mail.go:47`,
`internal/appbuild/appbuild_fs.go:92`).

**Verified during design review:**
- `os/exec` inheritance: `internal/cmdexec/cmdexec.go:264` never sets `ec.Env`,
so children inherit the full parent environment. Reproduced with a standalone
program — a child `sh -c` printed a parent-set variable. This is what makes the
credentials-directory source materially better than `password_env`.
- Mode bitmask: `mode.Perm()&0o077` is nonzero for 0644 (0044) and 0640 (0040),
zero for 0600. Verified by running it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`readFile` in `internal/secrets/secrets.go` already isolates "where the bytes
come from" from "how they are parsed". Both changes land there, so the two
callers (`internal/lua/context.go:35`, `internal/mail/config.go:241`) are
untouched.

1. **Source selection, project-scoped.** If `CREDENTIALS_DIRECTORY` is set AND
contains a credential file for *this project*, use it; else the project
`.rela/`. The credential filename is derived from the project so a
process-global directory cannot serve one tenant's secrets to another
(RR-Y2O7C6). Absent that file, fall back silently.
2. **Permission warning, de-duplicated.** `os.Stat` the chosen path; if
`mode.Perm()&0o077 != 0`, emit one `slog.Warn` with the path, octal mode, and
the fixing command. Suppressed after the first observation of a given path via a
package-level `sync.Map` (RR-V1G22E), with an unexported reset hook for tests
(RR-8Z97UM). Skipped for the credentials directory.

`internal/secrets` is a leaf with no `deps` entry in `.go-arch-lint.yml`, so it
stays stdlib-only. `log/slog` and `sync` are stdlib; no new dependency and no
arch-lint change.

Windows is a no-op case: Go reports synthetic modes there, so gate the check on
`runtime.GOOS != "windows"` to avoid a warning nobody can act on.

**Files to modify:**
- `internal/secrets/secrets.go` — source selection + permission check
- `internal/secrets/secrets_test.go` — new tests
- `docs/lua-scripting.md` — secrets Configuration section
- `docs/mail.md` — systemd guidance beside the existing `password_env` text

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `CREDENTIALS_DIRECTORY` — set by systemd, trusted to the same degree as the
process environment generally (anything that can set it can already set
`LD_PRELOAD`). Used as a directory path only; the filename is derived from our
own constant plus the project identity, never from request input, so there is no
traversal vector. Absent file → fall back, not fail.
- `secrets.yaml` contents — unchanged; still operator-authored YAML.

**Security-Sensitive Operations:**

- **The warning must never echo a secret.** It logs the path and the octal mode
only — no key names, no values. Existing `Load` errors already wrap the path and
not the content; that stays.
- **No mode-fixing.** `Chmod`-ing the operator's file on read would be a
surprising side effect from a read path, and would mask the misconfiguration
rather than surfacing it.
- **TOCTOU considered and dismissed.** The `Stat`-then-`ReadFile` pair is a
classic TOCTOU shape, but the check is *advisory logging*, not an access
decision — nothing is granted or denied on its result. An attacker who can swap
the file between the two calls already has write access to the secrets file,
which is game over independently. Explicitly NOT worth an `O_NOFOLLOW`
/`Fstat`-on-handle rewrite here; that would be justified only if the check ever
gates access.
- Note the *real* win is the credentials directory: `password_env` puts a
credential in the environment, which `internal/cmdexec` children provably
inherit (verified above). Credentials-directory secrets are not inherited.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Warning assertions capture `slog` output via a
`slog.New(slog.NewTextHandler(&buf, ...))` installed with `slog.SetDefault` and
restored by `t.Cleanup`. Every such test also calls the warn-cache reset hook in
`t.Cleanup`. **No `t.Parallel()` in tests using `t.Setenv`** — the runtime
panics on that pair (RR-8Z97UM).

1. AC1 — 0644 file → exactly one warn, secrets still returned.
2. AC2 — 0600 file → no warn.
3. AC3 — `CREDENTIALS_DIRECTORY` with a matching file → values come from it; a
deliberately different value in the project file proves precedence.
4. AC4 — `CREDENTIALS_DIRECTORY` set, no matching file → project file used.
5. AC5 — credentials-dir file at 0644 → no warn.
6. AC6 — three consecutive `Load` calls on one permissive path → exactly one
warn line.
7. AC7 — two distinct relaDirs + one `CREDENTIALS_DIRECTORY` → each project
gets its own secrets, neither sees the other's.

**Edge Cases:**

- `CREDENTIALS_DIRECTORY` set to empty string → treat as unset.
- `CREDENTIALS_DIRECTORY` pointing at a nonexistent dir → fall back, no error.
- Neither source exists → `ErrNotFound` as today (regression-guarded by the
existing `TestLoad_FileNotFound`).
- Symlinked secrets.yaml → `os.Stat` follows the link and reports the target's
mode, which is the mode that matters.
- `Stat` fails but `ReadFile` succeeds (race, or unreadable dir) → skip the
warning, do not fail the load.
- Windows → check skipped.

**Negative Tests:**

- Invalid YAML in the credentials dir must error, not silently fall back to the
project file — a fallback there would let a corrupt credential mask a working
file and produce confusing behaviour.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Warning fatigue.* `secrets.Load` runs per script execution and twice per mail
send — far more often than the original plan assumed (RR-V1G22E). Mitigated by
per-path warn-once, so a document-rendering server emits one line, not one per
render.
- *Cross-tenant secret bleed.* A process-global credentials directory served to
every project would hand one tenant another's secrets (RR-Y2O7C6). Mitigated by
project-scoping the credential filename.
- *Refusing would be safer but breaks deployments.* Chose warn; recorded as out
of scope rather than forgotten.
- *systemd behaviour unverified on this machine.* macOS dev box, no systemd. The
`$CREDENTIALS_DIRECTORY` contract is documented behaviour, and the code path is
a plain env-var + directory read that is fully unit-testable without systemd.
Docs will not claim a TPM2-backed setup was tested end-to-end.
- *Coverage floor.* `internal/secrets` has no override in `.testcoverage.yml`, so
it sits at the default 50. New error branches need tests (RR-8Z97UM).

**Effort:** s

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] `docs/lua-scripting.md` — secrets Configuration section: file permissions
and the credentials-directory source
- [x] `docs/mail.md` — systemd `LoadCredentialEncrypted=` beside the existing
`password_env` guidance, noting why it is preferable
- [x] ~~metamodel.md / cli-reference.md / data-entry.md / CLAUDE.md /
README.md~~ (N/A: no metamodel, UI, convention, or project-level change. The
`rela secrets credential-name` command added during implementation carries its
own kong `help:` text, and this repo has no cli-reference.md.)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**
- RR-V1G22E (significant) — warning fires per script execution / twice per mail
send; needs per-path warn-once. Plan updated.
- RR-Y2O7C6 (significant) — process-global `CREDENTIALS_DIRECTORY` would leak
secrets across tenants; needs project-scoped credential naming. Plan updated.
- RR-8Z97UM (minor) — warn cache is global mutable state; needs a test reset
hook, and `t.Setenv` forbids `t.Parallel`. Plan updated.
