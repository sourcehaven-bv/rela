---
id: TKT-XZ4EO
type: ticket
title: Relocate .rela/ user-local state to user config directory (cross-platform)
kind: enhancement
priority: high
effort: m
status: in-progress
---

## Problem

`.rela/` currently holds a mix of project-level state (cache, index) and
user-local state (age identity key, rendered document caches, UI state, user
defaults, scheduler state). The user-local pieces are plaintext by design, and
several are sensitive.

The threat model of the whole-repo encryption feature (FEAT-KAJBD) assumes the
repo directory may be synced to semi-trusted storage (Dropbox, iCloud, NAS).
`.gitignore` does not help there — directory-sync tools copy `.rela/` along with
everything else. Today the mitigation is docs-only: `rela keys init` prints a
warning, and `docs/encryption.md` says "do not sync `.rela/`."

Separately, the S1 rollback defense in the security review assumes
`last_seen_version` lives *outside* the repo. Shipping that defense without
relocation leaves a first-clone TOFU window that is effectively always open.

## Fix

Move user-local state out of `.rela/` and into the OS-native user config
directory, scoped per-repo by a stable fingerprint:

- Linux:   `$XDG_CONFIG_HOME/rela/repos/<fp>/` → `~/.config/rela/repos/<fp>/`
- macOS:   `~/Library/Application Support/rela/repos/<fp>/`
- Windows: `%AppData%\rela\repos\<fp>\`

Use Go's `os.UserConfigDir()` which returns the right path on each platform —
this is the standard cross-platform user-config abstraction.

Files that relocate:

- `key` (age identity)
- `last_seen_version` (for S1 rollback defense)
- rendered-document caches (previously `.rela/documents/`)
- `ui-state.json`
- `user-defaults.yaml`
- `palette.yaml`

`<fp>` is a stable fingerprint derived at `rela keys init` time (repo UUID baked
into `recipients.age`, or a hash of the initial recipient-list blob). The
fingerprint must NOT depend on the repo's on-disk path (users move directories)
and must NOT leak repo content.

`.rela/` keeps only project-derived artifacts safe to sync:
- `cache.json` (graph cache — rebuildable from entity files)
- `fsstore-index.json`
- `scheduler-state.json` (per-user last-run timestamps — TBD whether
this moves or stays; see planning)

## Cross-platform requirement

All three supported platforms (Linux, macOS, Windows) must work without
divergence in behavior or test coverage. The user config directory resolution,
path joining, and file permissions must use cross-platform primitives. Introduce
a thin user-state service interface so non-fs backends (e.g. test fakes) can
replace it.

## Scope

- **Unreleased feature** — FEAT-KAJBD hasn't shipped in a tagged
release. No migration from existing `.rela/` layouts needed; repos on disk will
either re-init or tolerate a one-shot auto-relocate.
- **Platform compliance** — honor `$XDG_CONFIG_HOME`, `%AppData%`,
and macOS `Application Support`. Use `os.UserConfigDir()`.

## Source

- `.ignored/encryption-security-review.md` C2 (relocation follow-up)
- `.ignored/encryption-security-review.md` S1 (`last_seen_version`
off-repo assumption)
- `.ignored/README.md` item 3 ("Relocate `.rela/` to XDG as a
separate small PR")

## Why now

- Closes the Dropbox/iCloud plaintext-sync leak that docs currently
only warn about.
- Unblocks S1's rollback defense on first-clone.
- Unreleased feature → no migration complexity.
- Isolated PR, small-medium effort.

## Blocked by

- PR #464 (merged 2026-04-20) — the at-rest encryption feature that
introduced the C2 / S1 findings.
