---
id: TKT-8S1SA
type: ticket
title: Refactor encryption into transparent FS decorator
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: ready
---

## Description

Encryption support is currently splattered across fsstore at eight named call
sites. fsstore holds a `Crypto` field, every read/write path goes through
`readFileUnsealed`/`writeFileSealed` helpers, and the factory wires a sentinel
`IdentityCrypto()` when encryption is off. Refactor so encryption becomes a
transparent decorator over a narrow filesystem interface — fsstore should not
know encryption exists.

## Problem

Encryption leaks into non-encryption code at eight call sites:

- `internal/store/fsstore/fsstore.go:58,95,146,158` — `Config.Crypto`, `s.crypto` field, `IdentityCrypto()` install, `verifyEncryptionConsistency` in `New`.
- `internal/store/fsstore/markdown.go:484–514` — `readFileUnsealed` / `writeFileSealed` helpers; the latter duplicates `SafeFS`'s MkdirAll + temp+rename.
- `internal/store/fsstore/attachment.go:33,45,69` — helpers plus manual MkdirAll.
- `internal/store/fsstore/index.go:88–92` — `savePersistedIndex` bypasses its own helpers and calls `s.crypto.Seal` + `s.fs.WriteFile` directly. Likely unintentional; `loadPersistedIndex` does use the helper.
- `internal/store/fsstore/watcher.go:179,192` — explicit raw ReadFile → hash → Unseal.
- `internal/store/fsstore/crypto_verify.go` — 186-line dedicated file walking fsstore dirs to classify files sealed/cleartext.
- `internal/app/factory.go:38–68` — `loadCrypto()` wiring.
- `internal/cli/show.go:71` — imports `internal/encryption` directly for `IsNoMatchingKey` / `IsCorrupted` / `IsNoPrivateKey` predicates.

Latent bug (not introduced by the branch):
`internal/store/fsstore/formatter.go:31,67` reads raw via `s.fs.ReadFile` and
compares to plaintext-formatted output. On encrypted repos that compare always
diffs and always rewrites.

## Goal

fsstore sees a plain byte-level filesystem interface. Encryption is a decorator
that wraps the filesystem layer. Caller wiring decides whether the decorator is
in the chain or not; fsstore has no crypto-awareness after the refactor.

## Proposed design

**Consumer-defined interface in fsstore:**

```go
type StoreFS interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm os.FileMode) error // auto-mkdir
    Remove(path string) error
    Rename(old, new string) error
    Stat(path string) (os.FileInfo, error)
}
```

Five methods. Excludes `ReadDir`, `Walk`, `Open`, `Getwd` — those are raw
directory topology, not backing bytes. fsstore keeps two fields: `bytes StoreFS`
(decorated) and `fs storage.FS` (raw, for dir enumeration, watcher, verifier,
temp cleanup).

**Layering:**

```
OsFS → SafeFS (atomic + mkdir) → EncryptedFS (seal/unseal) → fsstore
```

| Layer | Guarantees |
|---|---|
| SafeFS | atomic write, parent dir exists |
| EncryptedFS | ReadFile returns plaintext; WriteFile accepts plaintext |
| fsstore | entity/relation semantics, no crypto awareness |

**Cleartext mode = no decorator.** Factory passes `SafeFS` directly when
`.rela/encryption.yaml` is absent. No `IdentityCrypto()` sentinel, no
`isCleartextMode` introspection. The verifier gets an explicit `mode` argument
(`Cleartext`/`Encrypted`) from the factory instead of type-asserting.

## Hazards addressed (not waved away)

- **Watcher stays crypto-aware.** fsnotify is raw-byte by nature; self-echo hashing must happen on bytes as they hit disk. Refactor isolates this to one site instead of three, but watcher cannot be a pure-plaintext consumer.
- **Verifier stays crypto-aware.** It's a layer-crossing startup invariant. Moves **out** of fsstore into the factory or `storage/integrity`, not away.
- **Atomic-write ordering.** Stack must be `EncryptedFS(SafeFS(OsFS))`: seal happens first, SafeFS writes already-sealed bytes atomically. Crash leaves sealed temp, never plaintext. Note: `SafeFS.WriteFile` currently uses `os.OpenFile` directly (`safefs.go:40`) instead of delegating to its inner FS — pre-existing quirk; pins SafeFS to OS-backed inner. Acceptable for this refactor; flag separately.
- **Error classification survives.** `cli/show.go` uses `errors.Is`-compatible predicates; as long as `EncryptedFS` wraps with `fmt.Errorf("%w: ...", encryption.ErrNoMatchingKey)`, predicates still flow through. Add a CLI-level round-trip test to catch future `%s`-wrap regressions.
- **Attachments remain fully buffered.** Existing regression vector, not introduced by this refactor. Streaming `Open`/`Create` is a follow-up.

## Bonus fixes

- `savePersistedIndex` inconsistency disappears (it becomes plain `s.bytes.WriteFile`).
- `formatter.go` sealed-vs-plaintext compare bug fixes itself (reads via `s.bytes.ReadFile` return plaintext).

## Scope

**In scope:**

- New `StoreFS` interface in `internal/store/fsstore`
- New `EncryptedFS` decorator (location TBD during planning — `internal/encryption/fs.go` or a new `storage/cryptofs`)
- Remove `Config.Crypto`, `FSStore.crypto`, `identityCrypto`, `ageCrypto`, `readFileUnsealed`, `writeFileSealed`, `isCleartextMode` from fsstore
- Relocate `verifyEncryptionConsistency` out of fsstore into the wiring layer with explicit mode arg
- Fix `savePersistedIndex` and `formatter.go` bugs as side effects
- Fold `MkdirAll` into `WriteFile` contract; remove manual MkdirAll calls at call sites

**Out of scope:**

- Streaming attachment I/O
- Refactoring `SafeFS` to use `inner.WriteFile` instead of `os.OpenFile` directly
- Changing the wire format of sealed blobs
- Changing the key management CLI

## Acceptance criteria

1. Grepping `internal/store/fsstore` for `encryption`, `crypto`, `Seal`, `Unseal` returns zero hits outside the watcher (single `Unseal` call) and zero hits in `markdown.go`, `attachment.go`, `index.go`, `fsstore.go`.
2. `internal/store/fsstore` does not import `internal/encryption`.
3. Encryption on: full existing test suite passes; demo `demos/encryption/demo.sh` still produces sealed output and round-trips.
4. Encryption off: every `fsstore_test.go` test still passes; on-disk bytes byte-for-byte identical to pre-refactor cleartext output.
5. Consistency verifier still rejects half-migrated repos (both directions); lives outside fsstore.
6. CLI error classification (`classifyReadError` in `show.go`) still distinguishes no-matching-key / no-private-key / corrupted — new test round-trips `age.ErrIncorrectIdentity` through the full stack.
7. `savePersistedIndex` uses the same write path as every other write.
8. `formatter.go` no longer reports false diffs on encrypted repos — new regression test.

## Migration path (planning-phase refinement)

Each step keeps tests green:

1. Introduce `StoreFS` interface in fsstore; adapt `readFileUnsealed`/`writeFileSealed` callers to route through it; keep `Crypto` field. (pure plumbing)
2. Build the decorator; unit-test against MemFS. Bundle with step 1.
3. Swap wiring: factory builds the stack, fsstore takes `StoreFS`. Delete `Config.Crypto`, helpers. Fix `savePersistedIndex` and `formatter.go` bugs.
4. Relocate verifier out of fsstore with explicit mode arg. Bundle with step 3.
5. Consolidate `MkdirAll` into `WriteFile` contract; remove manual calls. Separate PR for audit.
