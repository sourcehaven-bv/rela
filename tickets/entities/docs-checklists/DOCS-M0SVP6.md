---
id: DOCS-M0SVP6
type: docs-checklist
title: 'Docs: Secrets hygiene: enforce 0600 on .rela/secrets.yaml and support systemd LoadCredentialEncrypted'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on new exported symbols
- [x] Rationale captured where a decision is non-obvious

`CredentialName` is the one new exported symbol. Its godoc states the name
format, why *both* halves are needed (a "# Why both halves are needed" section
naming the fail-open cross-tenant leak a basename-only name would cause), which
inputs are rejected and why, and points at `rela secrets credential-name`
instead of hand-derivation.

The package doc gained a "# Sources" section listing the two sources in
precedence order and why the systemd one is preferred (per-service tmpfs, 0400,
not inherited by children, TPM-bindable).

Unexported but load-bearing rationale is also recorded: `warnIfPermissive`
documents why the Stat/ReadFile TOCTOU is not security-relevant here (the check
gates nothing), why it warns rather than refuses, and why Windows is skipped;
`warnedPaths` documents the per-entry keying and the cap; `cacheDirName`
documents why the constant is duplicated rather than imported from
`internal/project` (arch-lint leaf rule).

## Project Documentation

- [x] `docs/lua-scripting.md` updated
- [x] `docs/mail.md` updated
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no such file in this repo — CLI help is
generated from kong tags, and the new command carries its own `help:`)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI change)
- [x] ~~`CLAUDE.md`~~ (N/A: introduces no new pattern or convention; it follows
the existing `slog.Warn`-for-degraded-config and leaf-package rules)
- [x] ~~`README.md`~~ (N/A: no project-level change)

`docs/lua-scripting.md` — the Secrets section now covers file permissions
(`chmod 700 .rela` **and** `chmod 600 .rela/secrets.yaml`, with the reason both
matter and an explicit statement that rela checks only the file), and gained a
"systemd credentials" subsection: how to get the name, `systemd-creds encrypt`,
the unit-file line, why it beats a plaintext file or an env var, and the note
that the name changes if the project directory moves.

`docs/mail.md` — the same permission correction in short form, plus a pointer to
the systemd section, and an explicit statement that `password_env` is the
weakest of the three because every spawned process inherits it.

## External Documentation

- [x] ~~Changelog~~ (N/A: not maintained in this repo)
- [x] ~~Migration notes~~ (N/A: purely additive — an existing
`.rela/secrets.yaml` keeps working unchanged. The only new user-visible
behaviour is a warning on an over-permissive file, which is advisory and does
not fail the load.)

## Accuracy

- [x] Examples were run, not written from memory

The `chmod`/warning behaviour and the credential precedence were exercised
end-to-end and the actual output is quoted in the implementation checklist. The
`rela secrets credential-name` example output was taken from a real run against
this repo's `tickets/` project and verified to equal what the loader derives.

**One example is NOT verified:** the `systemd-creds encrypt` command and the
`LoadCredentialEncrypted=` unit-file line. No systemd on this machine, so those
two lines come from systemd's documentation. Flagged on the ticket and in the
review checklist for validation on a Linux host.
