---
id: TKT-RX7I97
type: ticket
title: 'Secrets hygiene: enforce 0600 on .rela/secrets.yaml and support systemd LoadCredentialEncrypted'
kind: enhancement
priority: medium
effort: s
tags:
    - security
status: review
---

## Problem

Two gaps in how `.rela/secrets.yaml` is protected, found while evaluating sops
for rela secrets.

**1. No permission enforcement.** `internal/secrets/secrets.go` reads the file
with a plain `os.ReadFile` and never checks or sets its mode. There is no
`Chmod` and no `Stat` mode check anywhere in the package — the `0600` only
appears in a test fixture (`secrets_test.go:127`). rela never writes the file
itself (it is operator-authored), so the mode is whatever the operator's editor
and umask produced. On a permissive umask it can land group- or world-readable
and nothing says so. `.rela/` itself is created `0755`
(`internal/project/context.go:142`).

**2. No documented story for injecting secrets on a server.** `docs/mail.md`
mentions systemd units only in the context of `password_env`, which puts the
credential in the process environment — inherited by every child process,
including the third-party converters `internal/cmdexec` runs over
attacker-influenceable bytes. systemd's `LoadCredentialEncrypted=` is strictly
better on that axis and is currently undocumented.

## Why not sops / at-rest encryption

Investigated and rejected for this ticket (see the investigation notes below).
Encrypting `secrets.yaml` and handing the process a key via `SOPS_AGE_KEY` does
not strictly improve hygiene: it trades N secrets in a file for one master key
in the environment, which is a worse location because env vars are inherited by
subprocesses and appear in `/proc/PID/environ`. Using `SOPS_AGE_KEY_FILE`
instead lands back at "a plaintext credential in a 0600 file" — exactly what we
already have.

Note the `go.mozilla.org/sops` import path is dead regardless: the module
declares itself as `github.com/getsops/sops/v3` and Go refuses the old path. The
real library links all five KMS backends unconditionally (AWS, Azure, GCP,
Vault) — 139 modules, ~58 MB of linked code, roughly doubling the `rela` binary.

At-rest encryption of repo *data* remains covered by [[repo-encryption]] /
DEC-D5P4X; this ticket is only about the secrets file's hygiene.

## Scope

**1. Permission check on load.** In `internal/secrets`, `Stat` the file before
reading and warn when it is group- or world-readable. Warn rather than refuse:
the file is operator-authored and failing closed would break existing working
deployments on upgrade. Where rela does create the file's directory, prefer
`0700`.

**2. systemd credentials support.** Read `$CREDENTIALS_DIRECTORY` as a secrets
source when set, so `LoadCredentialEncrypted=` works without materializing a
plaintext file in the project. Secrets land in a per-service tmpfs, mode 0400,
owned by the service user, not inherited by children, with keys optionally
TPM2-backed. Document it in `docs/mail.md` and `docs/lua-scripting.md`.

The `secrets.Load` seam has only two non-test callers
(`internal/lua/context.go:35`, `internal/mail/config.go:241`), so the source of
the bytes stays an implementation detail.

## Out of scope

- Encrypting `secrets.yaml` at rest (rejected above).
- macOS Keychain as a third backend.
- Refusing to load a world-readable file (warn only, for now).

## Verification

`systemd-creds` behaviour has not been verified hands-on — the investigation ran
on macOS. The `$CREDENTIALS_DIRECTORY` plumbing must be validated on a real
Linux host before the docs claim it works.
