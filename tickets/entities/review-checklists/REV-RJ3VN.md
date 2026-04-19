---
id: REV-RJ3VN
type: review-checklist
title: 'Review: Add internal/encryption crypto primitives (slice 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

Cranky-code-reviewer surfaced six significant findings and several
minors/nits. All significant were addressed directly in the
implementation (no `review-response` entities filed — they were
fixed-in-place):

- **S1**: Added `TestResolvePrivateKeyPath_ProjectLocalBeatsHome` —
  asserts project-local key wins when a home default also exists.
- **S2**: Added `TestErrDecryptGCM_DoesNotWrapCause` — guard against
  a future refactor wrapping the GCM cause and leaking diagnostics
  into error messages.
- **S3**: Deleted dead `TestParsePrivateKeyPEM_BadMLKEMSeed`; replaced
  with boundary-length tests (`privatePayloadSize ± 1` rejected).
- **S4**: Tightened `TestUnwrapKey_CrossKey` to require `ErrDecrypt`
  exclusively — `ErrBadBlob` was an "acceptable" escape hatch masking
  a real bug.
- **S5**: Added `TestWrap_SameRecipient_DistinctBlobs` — asserts
  wrapping the same data key for the same recipient twice produces
  different blobs.
- **S6 + M4**: `LoadKeyring` now rejects duplicate recipient
  identities (case-insensitive FS case); extracted the loop body into
  `loadRecipient` to satisfy `nestif`.

Minors/nits addressed:

- **M1**: Expanded the all-zero-nonce comment in `wrap.go` to call
  out nonce-reuse risk for any future contributor tempted to reuse
  the KEK.
- **M3**: Added baseline-parse assertion in
  `TestParsePublicKeyPEM_InvalidMLKEM` so a stdlib behaviour change
  can't silently neuter the test.
- **M5**: Renamed `TestLoadKeyring_ReadDirPermissionError` to
  `TestLoadKeyring_NonDirArg`.
- **N2**: Added a convention comment at the top of `errors.go`
  explaining the `%w`-sentinel / `%s`-cause pattern.
- **N4**: Added a comment on `wrapMagic = "RLAE"` explaining the
  anagram (collision avoidance with files that start with "RELA").

Not addressed (with reason):

- **M2** (log home-dir error at warn): would require importing
  `log/slog`, breaking the package's pure-stdlib-crypto surface. Left
  as a follow-up if the silent-failure turns out to bite in practice.
- **N1** (`TestMustLen_Passes` signature): reviewer's suggestion to
  add `t.Helper()` to a test (not a helper) is incorrect; blanked-out
  `_ *testing.T` is the right Go style when the body doesn't need `t`.
- **N3** (`projectRelaDir` constant use): minor enough to skip; the
  constant is referenced from both production (`loader.go`) and tests.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All 13 acceptance criteria from TKT-16RY1 have matching tests, all passing:

1. PEM round-trip — `TestMarshalPrivateKeyPEM_RoundTrip` PASS
2. Wrap/unwrap round-trip — `TestWrapKey_RoundTrip` PASS
3. Cross-key unwrap fails — `TestUnwrapKey_CrossKey` PASS
4. Seal/Open round-trip — `TestSealOpen_RoundTrip` PASS
5. Tamper detection — `TestOpen_Tamper`, `TestUnwrapKey_Tamper*` PASS
6. Blob parser rejections — `TestUnwrapKey_BadLength/BadMagic/BadVersion` PASS
7. PEM parser rejections — `TestParse{Private,Public}KeyPEM_*` PASS
8. LoadKeyring recipients — `TestLoadKeyring_*` PASS
9. LoadFromDir precedence — `TestResolvePrivateKeyPath_*` PASS (incl. new ProjectLocalBeatsHome)
10. Keyring accessors — `TestLoadKeyring_SingleRecipient`, `TestKeyring_Unwrap_NoPrivateKey` PASS
11. Redaction — `TestRedaction_NoLeaks`, `TestSecretTypes_NoStringMethods` PASS
12. NewDataKey determinism — `TestNewDataKey_DeterministicWithFixedReader` PASS
13. Coverage — 97.8% (floor 95% passes; 4 uncovered are FS-race paths)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: slice 1 is pure library with no user-facing surface yet; docs will be added in later slices that wire encryption into CLI/data-entry)
- [x] ~~User-facing documentation updated~~ (N/A: no user-facing changes in slice 1)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A for slice 1

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/404
