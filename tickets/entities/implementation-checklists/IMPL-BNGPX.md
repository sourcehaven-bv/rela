---
id: IMPL-BNGPX
type: implementation-checklist
title: 'Implementation: Refactor encryption into transparent FS decorator; switch X25519 → Hybrid'
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

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

All 15 acceptance criteria verified:

- **AC1** (no encryption imports in fsstore data-write sites): `internal/store/fsstore/{fsstore,markdown,attachment,index,formatter}.go` grep-clean for `internal/encryption`, `Seal`, `Unseal`. Only watcher.go retains an `encryption.IsCorrupted` import (the documented exception).
- **AC2** (StoreFS and DirFS interfaces in fsstore): declared in `internal/store/fsstore/interfaces.go`. FSStore holds `bytes StoreFS` and `fs storage.FS` (DirFS-compatible).
- **AC3** (no raw byte I/O through dir handle): only `watcher.go` calls `s.fs.ReadFile` (raw bytes for self-echo hashing, by design). No `s.fs.WriteFile` calls remain in fsstore.
- **AC4** (SafeFS PostWrite hook fires once per successful rename): `TestSafeFS_OnPostWrite_FiresWithBytesOnDisk` + `_DoesNotFireOnFailedWrite` + `_ReplacesPreviousObserver` + `_NoObserverIsFineByDefault` all pass.
- **AC5** (watcher self-echo test through full encrypted stack): `TestSelfWriteIsSuppressed` passes with the refactored pipeline.
- **AC6** (encryption-on suite + demo round-trip): `go test ./... -race` all green; `demos/encryption/demo.sh` completes end-to-end.
- **AC7** (cleartext golden-file assertions unchanged): all existing fsstore tests pass without modification.
- **AC8** (consistency verifier relocated out of fsstore): lives in `internal/storage/integrity`; fsstore's thin wrapper delegates. 9 dedicated tests in `integrity/verify_test.go`.
- **AC9** (single-branch decision for wantSealed + EncryptedFS): factory.go:50-59 — one `if wantSealed { bytes = cryptofs.New(...) }` block, same `wantSealed` flows into `fsstore.Config.WantSealed`.
- **AC10** (CLI error classification round-trip): existing `cli/show.go:71` predicates use `errors.Is`-compatible wrapping in `encryption/errors.go`; `cryptofs.FS.ReadFile` returns wrapped errors from `encryption.Unseal` which preserve the `IsNoMatchingKey`/`IsCorrupted`/`IsNoPrivateKey` classifiers. Verified via `TestReadFile_*ClassifiedViaErrorsIs` tests in cryptofs.
- **AC11** (savePersistedIndex uses s.bytes.WriteFile): `index.go:90`.
- **AC12** (formatter.go no false diffs on encrypted repos): regression tests unskipped, `TestFSStore_Encrypted_FormatEntity_NoFalseDiff` + `_FormatRelation_NoFalseDiff` pass.
- **AC13** (all age.*X25519* replaced): grep confirms zero hits in internal/encryption.
- **AC14** (`rela keys generate` produces a hybrid identity; status works): demo script verified end-to-end.
- **AC15** (--pub → --pub-file): flag renamed, help text explains 2 KB size reason, `TestReadRecipientFromFile` covers happy/empty/missing/garbage.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
