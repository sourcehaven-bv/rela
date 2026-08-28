---
id: RR-MHW4HZ
type: review-response
title: Credential name changed across a symlinked path, breaking symlink-swap deploys
finding: |-
    Hashing the absolute path fixed the basename collision but introduced a new failure: filepath.Abs does not resolve symlinks, and project.Discover (internal/project/context.go:86) only calls Abs, so Context.CacheDir carries whatever path the user supplied.

    Verified by running the binary from both sides of a symlink: the two paths stayed unresolved and hashed differently. A /srv/current -> /srv/releases/N deploy layout — the standard symlink-swap pattern — would therefore change the credential name on every release. The operator's unit file keeps the old name, no credential matches, and rela silently falls back to the project file or to password_env. Silent fallback on a credential is exactly the failure mode this ticket set out to remove.

    Found by testing the new code against a symlinked layout rather than by reading it; neither my own review nor the code review caught it, because it only appears once the name became path-derived.
severity: significant
status: addressed
---

## Resolution

`CredentialName` now calls `filepath.EvalSymlinks` after `filepath.Abs`, so a
project reached through a symlinked parent hashes to the same name as its
target.

Best-effort by design: if `EvalSymlinks` fails — most likely because the
directory does not exist yet — the unresolved absolute path is kept rather than
returning `""`. Losing the credential entirely over an unresolvable path would
be a worse failure than a name that is merely stable-but-unresolved, and the
`.rela` directory exists in every real deployment.

`TestCredentialName/symlinked path resolves to the same name as its target`
builds a real symlinked layout and asserts both paths agree. It skips rather
than fails where symlinks are unavailable.
