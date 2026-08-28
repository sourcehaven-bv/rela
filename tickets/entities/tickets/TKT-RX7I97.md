---
id: TKT-RX7I97
type: ticket
title: 'Secrets hygiene: enforce 0600 on .rela/secrets.yaml and support systemd LoadCredentialEncrypted'
kind: enhancement
priority: medium
effort: s
tags:
    - security
status: done
---

## Problem

Two gaps in how `.rela/secrets.yaml` is protected, found while evaluating sops
for rela secrets.

**1. No permission enforcement.** `internal/secrets/secrets.go` read the file
with a plain `os.ReadFile` and never checked its mode. There was no `Chmod` and
no `Stat` mode check anywhere in the package — the `0600` appeared only in a
test fixture. rela never writes the file itself (it is operator-authored), so
the mode is whatever the operator's editor and umask produced. On a permissive
umask it can land group- or world-readable and nothing said so. `.rela/` itself
is created `0755` (`internal/project/context.go:142`).

**2. No documented story for injecting secrets on a server.** `docs/mail.md`
mentioned systemd units only in the context of `password_env`, which puts the
credential in the process environment — inherited by every child process,
including the third-party converters `internal/cmdexec` runs over
attacker-influenceable bytes. That inheritance was confirmed empirically:
`cmdexec` never sets `ec.Env` (`internal/cmdexec/cmdexec.go:264`), so children
receive the full parent environment. systemd's `LoadCredentialEncrypted=` is
better on exactly that axis and was undocumented.

## Why not sops / at-rest encryption

Investigated and rejected. Encrypting `secrets.yaml` and handing the process a
key via `SOPS_AGE_KEY` does not strictly improve hygiene: it trades N secrets in
a file for one master key in the environment, which is a worse location for the
inheritance reason above. Using `SOPS_AGE_KEY_FILE` instead lands back at "a
plaintext credential in a 0600 file" — what we already have.

The `go.mozilla.org/sops` import path is dead regardless: the module declares
itself as `github.com/getsops/sops/v3` and Go refuses the old path. The real
library links all five KMS backends unconditionally (AWS, Azure, GCP, Vault) —
139 modules, ~58 MB of linked code, roughly doubling the `rela` binary.

At-rest encryption of repo *data* remains covered by [[repo-encryption]] /
DEC-D5P4X; this ticket is only about the secrets file's hygiene.

## What shipped

**1. Permission warning.** `internal/secrets` stats the file and logs one
`slog.Warn` naming the path, octal mode, and the `chmod 600` fix when it is
group- or world-readable. Advisory: the file still loads, because failing closed
would break working deployments on upgrade. De-duplicated on `(path, mode)` and
capped at 1024 entries — `Load` runs per script execution and twice per mail
send, so a naive warning would fire on every document render, while a path-only
key would miss a fixed-then-regressed file.

**2. systemd credentials.** `Load` prefers `$CREDENTIALS_DIRECTORY` when it
holds a credential for *this* project. The name is
`rela-secrets-<project>-<hash>`, hashing the symlink-resolved absolute path:
`CREDENTIALS_DIRECTORY` is process-global while secrets are per-project, and a
basename-only name would serve one tenant's credentials to another, failing
open. Since the name is no longer derivable by hand, **`rela secrets
credential-name`** prints it.

**3. Docs.** `docs/lua-scripting.md` (permissions + a systemd-credentials
section) and `docs/mail.md`.

Both callers of the `secrets.Load` seam were left in place, so the source of the
bytes stayed an implementation detail — except in `internal/mail`, where
`resolvePassword` was discarding every error identically and would have reported
"password_env is empty or unset" for a corrupt credential. It now distinguishes
an absent source from a broken one.

## Out of scope

- Encrypting `secrets.yaml` at rest (rejected above).
- macOS Keychain as a third backend.
- Refusing to load a world-readable file (warn only).
- Checking the mode of `.rela/` itself — the docs now tell operators to
`chmod 700 .rela` and state plainly that rela checks only the file.

## Verification gap

**`systemd-creds` was never run.** The investigation and implementation happened
on macOS, so the `$CREDENTIALS_DIRECTORY` contract was simulated (an env var
plus a correctly-named file) rather than exercised through a real unit. The
rela-side path is a plain env-var + directory read and is fully unit-tested, but
the `systemd-creds encrypt` and `LoadCredentialEncrypted=` lines in the docs
come from systemd's documentation. Validate them on a Linux host before relying
on them verbatim.
