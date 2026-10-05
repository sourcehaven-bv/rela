---
id: REV-E9JR23
type: review-checklist
title: 'Review: entity.Ref and one gated resolver in internal/visibility'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): green on each Stage 1 PR (#1712, #1716 to
  #1724) and on the BUG-BZQQDP PR
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): package and total floors pass

**Comment findings.** No advisory finding introduced was left unaddressed.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent): once per
PR, with the security reviewer alongside
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 74 linked, none open: 63 addressed, 7 wont-fix (RR-1BW3I7,
RR-48NUVN, RR-6NW78C, RR-G7E9IM, RR-SKLS4D, RR-U80O2G, RR-X5ZFJR), 4 deferred
with a reason (RR-1JGEEO, RR-59MRU6, RR-6G3KI7, RR-CJJUV8). The relation-surface
findings of the last PR are on BUG-BZQQDP (REV-DENHHG).

All: RR-0QNNXI, RR-0SD5ER, RR-12WFBV, RR-1BW3I7, RR-1D2VI9, RR-1JGEEO,
RR-1OASDL, RR-1P88UJ, RR-2IK76Z, RR-2JJ55O, RR-2KK5SV, RR-3QRVE5, RR-3YAKUC,
RR-48NUVN, RR-4PVCPI, RR-53BLX0, RR-59MRU6, RR-5R9EO1, RR-6G3KI7, RR-6NW78C,
RR-7XH3EG, RR-84QY70, RR-921YW5, RR-AWIMJ3, RR-BF7Q4A, RR-BR080F, RR-CJJUV8,
RR-CKCGMZ, RR-EV48RR, RR-FE1EGP, RR-G7E9IM, RR-GS8Y15, RR-HNCM4I, RR-I6VO23,
RR-J8O28W, RR-KBTP47, RR-KF9C87, RR-KH1QYJ, RR-LCG8J1, RR-M3D3GG, RR-NH0A1W,
RR-NHGD48, RR-NKQA5F, RR-NM7E86, RR-O8MIRI, RR-OGJ0NW, RR-PBH6L2, RR-POJSNL,
RR-QZCY6M, RR-R6FYL4, RR-S1UY0F, RR-S4S8ZG, RR-SCRYQI, RR-SKLS4D, RR-TG5ZBC,
RR-TYJON4, RR-U80O2G, RR-UITNVE, RR-VFH4YX, RR-VMCBPB, RR-W9PIZB, RR-X5ZFJR,
RR-X9KS99, RR-XD7YN9, RR-XFEB3J, RR-XVJT46, RR-XXXIV9, RR-XYYWZ7, RR-Y2WKG0,
RR-YMFO8M, RR-YWTQSJ, RR-YXB2C4, RR-Z23T2T, RR-ZK4JNW

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** (criteria from PLAN-Y1JVSB)

1. PASS: `internal/archguard/zeroface_test.go` passes; its allowlist holds only
write-prep entries with reasons, and may only shrink.
2. PASS: `internal/visibility/resolver_test.go` (`TestResolver_GateOrder`,
`TestResolver_Family`); per-surface 404s in `relation_faces_test.go`,
`attachment_face_test.go`, `history_face_test.go`.
3. PASS: `worldparity_test.go` and `TestResolver_LoadFailureIsAMissAndOneWarning`.
4. PASS: `TestFacedAttachment_*` (upload stays on its face, reference-counted
delete, face delete drops unreferenced bytes).
5. PASS: `TestFileProperty_OrdinaryWriteIs*` returns 422; cross-face download
is a 404.
6. PASS: `TestRelationGet_EdgeOnAnotherFaceIsNotFound`, `endpoints_test.go`
for `FilterRelations`, and `commands_face_test.go`/`gantt_face_test.go` for
command payloads and the gantt (BUG-BZQQDP).
7. PASS: `TestFacedHistory_*` over HTTP; CLI addressing in
`internal/cli/history_address_test.go`.
8. PASS: e2e `faces-history.spec.ts`, `faces-browse.spec.ts` and
`faces-reader.spec.ts` on the faced fixture.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-TDLNNN

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI: one PR per stage step,
all merged into faces-intrinsic except the BUG-BZQQDP PR
