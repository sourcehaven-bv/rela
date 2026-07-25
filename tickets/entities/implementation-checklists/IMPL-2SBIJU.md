---
id: IMPL-2SBIJU
type: implementation-checklist
title: 'Implementation: Native in-process image processing: decode-verify, EXIF-orientation, re-encode (Phase 1)'
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
Implemented across `internal/imgproc/` (new pure-Go package),
`internal/metamodel` (`image:` transform step + loader validation),
`internal/attachment` (native step wiring, filename-extension swap, runner-nil
path). All 13 ACs covered by tests:

- AC1 re-encode png/jpeg/gif/webp in-process, no runner → `TestPolicy_NativeImage_RunsWithNilRunner`, imgproc format table.
- AC2 EXIF orientation applied by default → `TestNormalize_AppliesOrientation_EndToEnd`, all-8-orientation parse + rotate tests.
- AC3 metadata stripped → `TestNormalize_StripsMetadata`, `TestPolicy_NativeImage_StripsMetadata`.
- AC4 pixel-cap bomb guard → `TestNormalize_PixelCap` (rejects from header, no alloc).
- AC5 crafted-input safety → `FuzzNormalize` (380k+ execs clean), panic-recover, `-race` green.
- AC6 WebP → PNG/JPEG re-encode documented + asserted.
- AC7 undecodable rejected → `TestPolicy_NativeImage_UndecodableRejected`.
- AC8 CLI parity: `rela attach` uses the same nil-runner PolicyProcessor → native steps run there.
- AC9 loader rejects malformed steps (both/neither/bad reencode/non-file) → metamodel image-transform tests.
- AC10 arch-lint, plimsoll, coverage floors, govulncheck all pass; x/image → v0.44.0.
- AC11 filename extension swapped on re-encode → `TestSwapExt`, integration `.jpg`/`.png` assertions.
- AC12 concurrency semaphore bounds decodes (process-wide) → package semaphore + goleak test.
- AC13 animated GIF rejected, single-frame re-encodes → `TestNormalize_AnimatedGIFRejected`, `TestNormalize_SingleFrameGIF`.

Quality gates: `just arch-lint` OK, `just plimsoll` OK, `just coverage-check`
PASS (imgproc 82.9% vs 78% floor), `just govulncheck` clean, full `go test ./...`
green (73 packages), `golangci-lint` 0 issues on all touched packages, builds on
default/memory/postgres tags. Committed on branch
`feat/native-image-processing-TKT-LNQX4I` (9064ac82).

Note: a pre-existing `FuzzParse` finding in `internal/metamodel` (a `!!` YAML tag
leaking into an "unknown key" error message) was surfaced during testing; it is
unrelated to this change (my diff does not touch YAML key parsing or that error
path) and is left for a separate ticket.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
