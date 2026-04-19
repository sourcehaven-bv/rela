---
id: TKT-8S1SA
type: ticket
title: Refactor encryption into transparent FS decorator; switch X25519 → Hybrid
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: done
---

## Description

Two related corrections to the encryption work on this branch:

1. **Refactor encryption into a transparent FS decorator.** Encryption currently leaks into fsstore at eight named call sites. Make fsstore unaware of encryption by wrapping the filesystem interface instead of threading a `Crypto` field through fsstore.
2. **Switch from `X25519Identity` to `HybridIdentity`.** The branch picked the classical-only API when it should have picked the post-quantum hybrid. Trivial, symmetric-API fix — correctness gap now, migration cost later.

---

## Part 1: Refactor to transparent FS decorator

### Problem

Encryption leaks into non-encryption code at eight call sites:

- `internal/store/fsstore/fsstore.go:58,95,146,158` — `Config.Crypto`, `s.crypto` field, `IdentityCrypto()` install, `verifyEncryptionConsistency` in `New`.
- `internal/store/fsstore/markdown.go:484–514` — `readFileUnsealed` / `writeFileSealed` helpers; the latter duplicates `SafeFS`'s MkdirAll + temp+rename.
- `internal/store/fsstore/attachment.go:33,45,69` — helpers plus manual MkdirAll.
- `internal/store/fsstore/index.go:88–92` — `savePersistedIndex` bypasses its own helpers and calls `s.crypto.Seal` + `s.fs.WriteFile` directly. Likely unintentional; `loadPersistedIndex` does use the helper.
- `internal/store/fsstore/watcher.go:179,192` — explicit raw ReadFile → hash → Unseal.
- `internal/store/fsstore/crypto_verify.go` — 186-line dedicated file walking fsstore dirs to classify files sealed/cleartext.
- `internal/app/factory.go:38–68` — `loadCrypto()` wiring.
- `internal/cli/show.go:71` — imports `internal/encryption` directly for `IsNoMatchingKey` / `IsCorrupted` / `IsNoPrivateKey` predicates.

Latent bug now promoted to an explicit scope item (see below):
`internal/store/fsstore/formatter.go:31,67` reads raw via `s.fs.ReadFile` and
compares to plaintext-formatted output. On encrypted repos that compare always
diffs and always rewrites.

### Goal

fsstore sees a plain byte-level filesystem interface. Encryption is a decorator
that wraps the filesystem layer. Caller wiring decides whether the decorator is
in the chain or not; fsstore has no crypto-awareness after the refactor.

### Proposed design

**Consumer-defined interface in fsstore:**

```go
// StoreFS is the byte boundary fsstore uses for every data read/write.
// Transforms (encryption, future compression) are composed above this
// interface; fsstore never knows which transforms are active.
type StoreFS interface {
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm os.FileMode) error
    Remove(path string) error
    Rename(old, new string) error
    Stat(path string) (os.FileInfo, error)
}
```

Five methods. Excludes `ReadDir`, `Walk`, `Open`, `Getwd` — those are raw
directory topology, not backing bytes.

**Second, narrower FS handle for dir enumeration.** fsstore *also* needs
directory-level operations (walk temp-file cleanup, scan entity dirs, stat
mtimes). Those go through a separate, deliberately read-only interface that
excludes `ReadFile`/`WriteFile`/`Open`:

```go
// DirFS is the raw directory view fsstore uses for enumeration,
// stat, and temp-file cleanup. It deliberately omits ReadFile/
// WriteFile/Open so that byte I/O is forced through StoreFS and
// cannot silently bypass transforms above it.
type DirFS interface {
    ReadDir(path string) ([]os.DirEntry, error)
    Stat(path string) (os.FileInfo, error)
    Walk(root string, fn filepath.WalkFunc) error
    Remove(path string) error
}
```

fsstore holds `bytes StoreFS` (transformed path) and `dirs DirFS` (raw
topology). The compiler enforces that no data bytes bypass the transform stack.

**Layering:**

```
OsFS → SafeFS (atomic + mkdir + PostWrite hook) → EncryptedFS (seal/unseal) → fsstore
```

| Layer | Guarantees |
|---|---|
| SafeFS | atomic write, parent dir exists, fires `PostWrite(path, bytesOnDisk)` exactly once per durable write |
| EncryptedFS | ReadFile returns plaintext; WriteFile accepts plaintext |
| fsstore | entity/relation semantics, no crypto awareness, subscribes to `PostWrite` for self-echo hashing |

**Cleartext mode = no decorator.** Factory passes `SafeFS` directly when
`.rela/encryption.yaml` is absent. No `IdentityCrypto()` sentinel, no
`isCleartextMode` introspection.

### Write-hook design (resolves self-echo across decorators)

The watcher's self-echo LRU hashes the bytes that sit on disk — that's what
fsnotify will give it on a re-read. After the refactor, the *writer* in fsstore
hands plaintext to `bytes.WriteFile` and never sees the sealed output, so
fsstore cannot record the right hash on its own.

**Solution: `PostWrite` callback on the lowest writer, not on EncryptedFS.**

```go
type WriteObserver func(path string, bytes []byte)

// SafeFS exposes a hook fired exactly once per successful durable
// write, with the bytes that landed on disk after the atomic rename.
// If the rename fails, the hook does not fire. fsstore registers a
// single observer that records hashContent(bytes) into recentHashes.
func (s *SafeFS) OnPostWrite(obs WriteObserver)
```

The contract is "hash what durably sits on disk," enforced by the only layer
that performs the OS write. Every transform above — EncryptedFS today, a future
compression or dedup decorator — is automatically covered because they all
bottom out in one SafeFS write. fsstore stays agnostic to the transform stack;
EncryptedFS stays ignorant of self-echo.

Fire-ordering: after successful `rename(tempPath, path)`. On rename failure, no
hook. On `Remove`: the watcher drops its hash via its existing `forgetHash`
path; `PostWrite` is write-only.

### Consistency-verifier relocation (concrete API)

Move `verifyEncryptionConsistency` out of fsstore into a new
`internal/storage/integrity` package with this signature:

```go
// Verify walks dirs and asserts every non-hidden file matches
// wantSealed. fs must be the RAW filesystem (not the decorated
// StoreFS), because verification inspects on-disk bytes.
// fs is the SAME `storage.FS` handle the factory passes to SafeFS
// so the "is this repo sealed?" answer cannot drift.
func Verify(fs storage.FS, wantSealed bool, dirs []string) error
```

Called by the factory **between** "FS stack assembled" and "fsstore.New." The
factory is the single place where `wantSealed` and "was EncryptedFS installed"
are both decided, in a single `if cfgExists` branch:

```go
wantSealed := cfgExists
var bytes StoreFS = safe
if wantSealed {
    bytes = cryptofs.Wrap(safe, ageCrypto)
}
if err := integrity.Verify(raw, wantSealed, []string{
    paths.EntitiesDir, paths.RelationsDir, paths.AttachmentsDir,
}); err != nil {
    return nil, err
}
return fsstore.New(fsstore.Config{Bytes: bytes, Dirs: raw, ...})
```

One branch, two uses — cannot drift.

### Hazards addressed

- **Watcher self-echo.** Solved by the `PostWrite` hook design above. Watcher keeps its raw-FS handle to read on fsnotify events; fsstore subscribes to SafeFS's hook and records `hashContent(bytesOnDisk)`. Watcher's `recentHashes.Get(path) == hashContent(data)` check works unchanged.
- **Verifier stays raw-byte-aware.** Relocated to `internal/storage/integrity` with explicit signature above. Operates on the raw FS handle; never touches the decorated stack.
- **Atomic-write ordering.** Stack is `EncryptedFS(SafeFS(OsFS))`: seal happens first, SafeFS writes already-sealed bytes atomically. Crash leaves sealed temp, never plaintext.
- **SafeFS pins to OsFS.** `SafeFS.WriteFile` uses `os.OpenFile` directly (`safefs.go:40`) — pre-existing. Acceptable because SafeFS now owns the `PostWrite` hook too, so it needs to own the full write (no delegation to inner). Test layering made explicit: EncryptedFS unit tests use raw MemFS (no SafeFS). Full-stack fsstore tests use `EncryptedFS(SafeFS(OsFS))` against `t.TempDir()`.
- **Error classification.** `cli/show.go:71` predicates (`IsNoMatchingKey` etc.) use `errors.Is` (see `encryption/errors.go:29,33,37`), so `EncryptedFS` wrapping with `fmt.Errorf("%w: ...", encryption.ErrNoMatchingKey)` propagates through transparently. Add CLI round-trip test to catch future `%s`-wrap regressions.
- **Attachments remain fully buffered.** Existing regression vector. Streaming `Open`/`Create` is a follow-up ticket.

### Bonus fixes (fall out of the refactor)

- `savePersistedIndex` inconsistency disappears (it becomes plain `s.bytes.WriteFile`).
- `formatter.go:31,67` bug fixes itself (reads via `s.bytes.ReadFile` return plaintext). Write a failing regression test first on current branch as `t.Skip("blocked on TKT-8S1SA")` so it cannot be forgotten; unskip at the end.

---

## Part 2: Switch X25519Identity → HybridIdentity

### Problem

`internal/encryption/identity.go` uses `age.GenerateX25519Identity`,
`age.ParseX25519Identity`, `age.ParseX25519Recipient` throughout. This is the
**classical-only** API. age's own documentation explicitly recommends the
post-quantum hybrid API for new integrations:

> For most use cases, use the Encrypt and Decrypt functions with HybridRecipient and HybridIdentity.

> When integrating age into a new system, it's recommended that you only support native (X25519 and hybrid) keys, and not SSH keys.

Picking X25519 for a brand-new encryption-at-rest feature is a correctness gap:
repos sealed with X25519 today cannot be upgraded to PQ-safe without rotating
every file. Fix now — the API shapes are symmetric.

### Byte-format impact

Hybrid recipients are **~1959 chars** with prefix `age1pq1...` (vs ~62 chars
`age1...`); hybrid identities use prefix `AGE-SECRET-KEY-PQ-1...` (vs
`AGE-SECRET-KEY-1...`). Every hardcoded string must be updated.

### Files affected (enumerated)

Code:
- `internal/encryption/identity.go` — all `x25519Recipient` / `x25519Identity` types, `GenerateIdentity`, `ParseRecipient`, `ParseIdentity`, `ReadIdentity` (type-assertion to `*age.HybridIdentity`)
- `internal/encryption/marshal.go:3` — `AGE-SECRET-KEY-1` comment reference
- `internal/encryption/identity_test.go:23,34` — leak-detection substring check (currently checks for `AGE-SECRET-KEY-1`)
- `internal/encryption/keyring.go`, `loader.go` — follow type renames
- `internal/cli/keys.go:64,116` — user-visible strings referencing `age1...` format
- `internal/store/fsstore/helpers_test.go:38` — hardcoded example key

Docs:
- `docs/cli-reference.md:96,104,115,126`
- `docs-project/entities/guides/GUIDE-cli-reference.md:102,110,121,132`
- `docs/encryption.md:56`
- `docs-project/entities/guides/GUIDE-encryption.md:62`

Demo:
- `demos/encryption/demo.sh` — regenerate with hybrid API

### UX concern: `--pub <string>` flag

Hybrid public keys are ~1959 chars. Pasting 2 KB on the command line is awkward.
Proposal: switch `rela keys add --pub <string>` to `rela keys add --pub-file
<path>` (or accept both for back-compat and print a hint when a long string is
pasted). **Decide during planning.**

### Back-compat: none

Feature is unreleased. X25519 keys exist only in internal test repos on this
unmerged branch. Branch testers regenerate with `rela keys generate`. No
detection branch, no migration code, no deprecation shim carried forward.

### Acceptance

- All `ParseX25519*` / `GenerateX25519*` calls replaced with `*Hybrid*` equivalents.
- `rela keys generate` emits a hybrid identity.
- `identity_test.go` leak detection updated to check the actual hybrid-redacted form.
- `demos/encryption/demo.sh` still round-trips end to end.
- Docs updated with correct hybrid prefixes.
- `--pub`/`--pub-file` CLI UX decision recorded in the ticket.

---

## Scope

**In scope:**

- Part 1: new `StoreFS` + `DirFS` interfaces in `internal/store/fsstore`; new `EncryptedFS` decorator; new `SafeFS.OnPostWrite` hook and SafeFS→fsstore wiring; remove `Config.Crypto` / `FSStore.crypto` / `identityCrypto` / `ageCrypto` / `readFileUnsealed` / `writeFileSealed` / `isCleartextMode` from fsstore; relocate `verifyEncryptionConsistency` to `internal/storage/integrity` with explicit signature; fix `savePersistedIndex` and `formatter.go` bugs (with committed-first regression tests).
- Part 2: replace all X25519 API calls with Hybrid; clear-error detection of old X25519 keys; enumerated doc/test updates; CLI UX decision for `--pub`.

**Out of scope:**

- Streaming attachment I/O (follow-up ticket).
- Refactoring `SafeFS` to delegate temp-writes to its inner FS (pre-existing design; the PostWrite hook lives in SafeFS so it needs to own the write).
- Folding `MkdirAll` into the `WriteFile` contract (separate refactor; not encryption-related).
- Changing the wire format of sealed blobs (both X25519 and Hybrid wrap in the same age envelope).

## Acceptance criteria

1. **No encryption imports in fsstore data-write sites.** `internal/store/fsstore/{fsstore.go, markdown.go, attachment.go, index.go, formatter.go}` do not import `internal/encryption` and contain no occurrences of `Seal`, `Unseal`, or `crypto`. `watcher.go` may import `internal/encryption` **only** for the `IsCorrupted` predicate.
2. `internal/store/fsstore` defines its own `StoreFS` and `DirFS` interfaces; holds `bytes StoreFS` and `dirs DirFS` fields.
3. `fsstore.FSStore.dirs` does not expose `ReadFile`, `WriteFile`, or `Open`. Compiler enforces no raw byte I/O.
4. `SafeFS` exposes a `PostWrite` hook fired exactly once per successful atomic rename; unit test asserts it fires with the bytes on disk and does not fire on rename failure.
5. Watcher self-echo test: write an entity through the full `EncryptedFS(SafeFS(OsFS))` stack, wait for fsnotify, assert NO re-parse / re-emit happens (hash-on-disk matches recorded).
6. Encryption on: full existing test suite passes; demo `demos/encryption/demo.sh` still produces sealed output and round-trips.
7. Encryption off: `fsstore_test.go` golden-file assertions unchanged.
8. Consistency verifier rejects half-migrated repos (both directions); lives in `internal/storage/integrity`; called by the factory, not by fsstore.
9. Factory decides `wantSealed` and "install EncryptedFS" in a **single** `if` branch — test asserts no code path constructs one without the other.
10. CLI `classifyReadError` round-trips `age.ErrIncorrectIdentity` / `ErrCorrupted` / `ErrNoPrivateKey` through the full stack; new test in `internal/cli`.
11. `savePersistedIndex` uses the same `s.bytes.WriteFile` path as every other write.
12. `formatter.go` no longer reports false diffs on encrypted repos; regression test committed first on the current branch as `t.Skip("blocked on TKT-8S1SA")`, unskipped as the last step.
13. All `age.*X25519*` references in `internal/encryption` replaced with `age.*Hybrid*`.
14. `rela keys generate` produces a hybrid identity; `rela keys status` works with hybrid keys.
15. `--pub` vs `--pub-file` CLI UX decision recorded in ticket and implemented.

## Migration path

Each step keeps tests green. Hybrid switch first (mechanical), then the refactor
layered on a correct foundation.

1. **Part 2** — swap X25519 API calls for Hybrid in `internal/encryption`; update tests, docs, demo; decide and implement `--pub` vs `--pub-file`.
2. Commit `formatter.go` regression test as `t.Skip("blocked on TKT-8S1SA")`.
3. Introduce `StoreFS` and `DirFS` interfaces in fsstore; adapt existing `readFileUnsealed`/`writeFileSealed` callers to route through `StoreFS`; keep `Crypto` field for now. (pure plumbing)
4. Add `SafeFS.OnPostWrite` hook; wire fsstore to subscribe; verify watcher self-echo still works. *Bundle with step 3.*
5. Build `EncryptedFS` decorator; unit-test against MemFS. Full-stack tests use `EncryptedFS(SafeFS(OsFS))` against `t.TempDir()`.
6. Swap wiring: factory builds `EncryptedFS(SafeFS(OsFS))` in a single `if` branch; fsstore takes `StoreFS` + `DirFS`. Delete `Config.Crypto`, helpers. Fix `savePersistedIndex` and unskip the formatter test.
7. Relocate verifier to `internal/storage/integrity` with the signature above; factory invokes it. *Bundle with step 6.*
