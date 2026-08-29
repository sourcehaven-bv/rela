---
id: IMPL-WBKVF3
type: implementation-checklist
title: 'Implementation: Secrets hygiene: enforce 0600 on .rela/secrets.yaml and support systemd LoadCredentialEncrypted'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Helpers added: `captureWarnings` (slog capture + warn-cache reset via
`t.Cleanup`), `writeMode` (explicit chmod, defeating umask), `writeCredential`
and `projectRelaDir` (realistic `<project>/.rela` layout, since `CredentialName`
derives the project from the parent directory). Credential filenames come from
`CredentialName(relaDir)` rather than a hardcoded string, so the tests cannot
drift from the implementation's naming.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran a temporary end-to-end program against a real `/tmp/relaverify/acme/.rela`
layout (removed afterwards):

```
--- 1) world-readable project file: expect a WARN ---
WARN secrets: file is readable by other users
  path=/tmp/relaverify/acme/.rela/secrets.yaml mode=0644
  fix="chmod 600 /tmp/relaverify/acme/.rela/secrets.yaml"
   loaded: map[demo_key:from-project-file] err=<nil>
--- 2) second load of same path: NO further warning ---   (none emitted)
--- 3) systemd credential for this project wins ---
   credential name: rela-secrets-acme
   loaded: map[demo_key:from-systemd]
--- 4) unrelated credential present: falls back to project file ---
   loaded: map[demo_key:from-project-file]
```

Confirms AC1 (warn + still loads), AC3 (credential wins), AC4 (unrelated
credential does not shadow), AC6 (warn-once).

Automated checks:
- `go test ./...` — full suite green
- `go test -race ./internal/{secrets,lua,mail,script}/...` — clean (the
warn-once cache is shared mutable state, so the race detector matters here)
- `just lint` — clean (9 findings fixed, none suppressed except one documented
gosec G703 `//nolint`; see Quality below)
- `just arch-lint` — OK, no warnings
- `just comment-lint` — clean across 11,077 comments
- `just plimsoll` — clean
- Coverage: `internal/secrets` at **96.1%** (floor is the default 50)

**Not verified:** systemd itself. This is a macOS box with no `systemd-creds`,
so `LoadCredentialEncrypted=` was exercised by simulating the contract
(`CREDENTIALS_DIRECTORY` + a file named `rela-secrets-<project>`) rather than by
a real unit. The rela-side code is a plain env-var + directory read, but the
unit-file syntax in the docs is from documentation, not a live run. Flagged on
the ticket for validation on a Linux host.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `slog.Warn` for degraded-but-working config matches
`internal/appbuild/mail.go:47`. `internal/secrets` stays a stdlib-only leaf
(`log/slog`, `sync`, `runtime` are stdlib), so `.go-arch-lint.yml` needed no
change.

Security notes:
- The warning logs path + octal mode only. `TestLoad_WarningNeverEchoesSecrets`
asserts no key name or value reaches the log.
- One `//nolint:gosec` on the `os.Stat` of the credentials path (G703, path
traversal via taint analysis). Read before suppressing: the taint source is
`CREDENTIALS_DIRECTORY`, which is systemd-supplied, and the filename is rela's
own constant plus the project directory — no request-controlled component. The
comment states this at the site.
- The `Stat`-then-`ReadFile` TOCTOU shape is documented as deliberate: the
check is advisory logging, gating nothing.

Scratch file `zz_manual_verify_test.go` was used for the evidence above and
deleted; `git status` confirms only the four intended files are modified.
