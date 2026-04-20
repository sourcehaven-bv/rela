---
id: TKT-5HPJC
type: ticket
title: Whole-file seal/unseal at fsstore I/O boundary via age
kind: enhancement
priority: high
effort: m
status: backlog
---

Wire the rewritten `internal/encryption` package into `internal/store/fsstore` so that every entity, relation, attachment, and derived cache file is sealed as an age blob. Depends on TKT-7XEFM.

## Interface contract

Consumer-owned on the fsstore side; defined in `internal/store/fsstore/crypto.go`.

```go
// Crypto is the fsstore-side view of the encryption boundary. It is
// always non-nil: an identityCrypto is installed when the repo is
// not encryption-enabled.
type Crypto interface {
    // Seal produces the on-disk blob for a marshalled file.
    Seal(plaintext []byte) ([]byte, error)

    // Unseal inverts Seal. Errors MUST be classifiable via the three
    // predicates IsNoMatchingKey / IsCorrupted / IsNoPrivateKey from
    // internal/encryption. No sentinel values; no collapse of tamper
    // into no-matching-key (see DEC-D5P4X and the prior C1 regression).
    Unseal(blob []byte) ([]byte, error)

    // LooksSealed reports whether the first bytes of blob match a sealed
    // envelope. Used only for the partial-encryption invariant check at
    // fsstore.New; not on the hot read path.
    LooksSealed(blob []byte) bool
}

// identityCrypto is installed when .rela/encryption.yaml is absent.
// Seal/Unseal are identity; LooksSealed is always false.
```

## Write path invariants

1. Marshal the entity/relation to bytes.
2. Call `crypto.Seal` on the whole blob.
3. Write sealed bytes to `<path>.new` via the existing temp-file helper.
4. Rename `<path>.new` to `<path>`.

Never write cleartext to `<path>.new` when encryption is enabled. This is the key invariant for crash safety: an interrupted write leaves a valid sealed blob on disk, never plaintext.

## Read path

1. Read raw file bytes.
2. Call `crypto.Unseal`. For identityCrypto, this is a no-op; for real crypto, this is `age.Decrypt`.
3. Parse the unsealed bytes as markdown + YAML frontmatter as today.

No magic-byte peek, no conditional unseal. The identityCrypto path does no work; the encrypted path always unseals. One code path for both.

## fsstore.New partial-encryption invariant

On construction, after loading the crypto implementation:

- If `crypto` is identityCrypto: scan entities/, relations/, attachments/ for any file whose first bytes look like an age blob (`age-encryption.org/v1\n`). If any found: fail with `ErrRepoHasSealedFilesButNoConfig`, listing offending files.
- If `crypto` is the real age Crypto: scan the same directories for files that do NOT start with the age header. If any found: fail with `ErrRepoHasCleartextFilesButEncryptionEnabled`.

This refuses to open a half-migrated repo. Migration (a separate ticket) must be atomic at the repo level via an in-progress marker.

## Advisory file lock

On `fsstore.New` when opening for writes, acquire an advisory lock on `.rela/lock` (flock on Unix, LockFileEx on Windows). Release on close. This rules out concurrent writers producing files sealed under different data keys.

## Attachments

Attachments (`attachments/<entityID>/<property>/<filename>`) flow through the same Seal/Unseal path. Sealed as opaque binary. Size leakage accepted per the concept threat model.

## Derived caches

`.rela/cache.json` and `.rela/fsstore-index.json` are sealed when encryption is enabled. Read-path unseals before JSON decode; write-path seals after JSON encode.

## Recipient drift warning

On fsstore.New, if the committed recipient list in `.rela/encryption.yaml` differs from what the loaded keyring would produce (recipient added but files not rewrapped, or removed but still listed), emit a single slog.Warn pointing at `rela keys sync`. Does not block open.

## Watcher integration

`internal/store/fsstore/watcher.go`: external-change events for sealed files unseal before reconciliation. Unseal failure (e.g. user edited the sealed file in vim) produces a slog.Warn and the event is dropped (not reconciled).

## What changes outside fsstore

- `internal/workspace/workspace.go`: loads crypto and passes it to fsstore.New. Installs identityCrypto if `.rela/encryption.yaml` absent.
- Arch-lint: `fsstore` gains `may-depend-on: encryption`.

## Acceptance criteria

1. Sealed entity files round-trip through fsstore: seal → write → read → unseal → semantically equal.
2. Tampered sealed file (flip one byte in the payload) surfaces an error for which `encryption.IsCorrupted(err)` is true, via the **production** fsstore read path (not via a test fake). This is a regression test against the prior C1 bug.
3. Unsealing with an identity not in the recipient set surfaces `encryption.IsNoMatchingKey`.
4. Unsealing with no identity loaded surfaces `encryption.IsNoPrivateKey`.
5. `fsstore.New` refuses to open a half-migrated repo (cleartext + sealed files under `.rela/encryption.yaml` presence/absence).
6. `fsstore.New` acquires `.rela/lock` on write-open; concurrent processes cannot both open for writes.
7. identityCrypto path produces byte-for-byte identical output to pre-feature fsstore.
8. Attachments seal/unseal through the same path.
9. `.rela/cache.json` and `.rela/fsstore-index.json` are sealed on encryption-enabled repos.
10. Watcher unseals sealed files; malformed-sealed files produce a warning, not a crash.
11. Drift between committed recipient list and loaded keyring produces a single slog.Warn on open.
12. `just ci` passes.

## Out of scope (separate tickets)

- `rela keys generate / add / remove / rotate / sync / migrate` CLI commands.
- One-shot cleartext-to-encrypted migration (with resumable state).
- PQ recipient plugin.
- Signed envelopes to protect against pubkey substitution.
