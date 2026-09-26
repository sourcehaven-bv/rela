---
id: REV-LA7S7I
type: review-checklist
title: 'Review: MCP attachment tools: list/read/attach/delete for agents, ACL-gated'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`: exit 0 after the review fixes)
- [x] Lint clean (`just lint`: 0 issues; `just arch-lint` and `just plimsoll` clean)
- [x] Comment lint gate clean (`just comment-lint`: no unresolvable doc links; the one advisory on EntityPatcher predates this diff)
- [x] Coverage maintained (`just coverage-check`: PASS, total 80.1%)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer and rela-security-reviewer)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (HTTP memory gate, stdio frame limit, remote wiring test)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-0LVR13 RR-1B9151 RR-20DN1Y RR-31CANV RR-47I4H6
RR-5109KC RR-6VQR9O RR-7FZ4BD RR-7I4002 RR-8ROBUI RR-9DDPMV RR-AE3JYO RR-CN70H8
RR-DNKOT0 RR-DXIJG9 RR-FAIX1V RR-H24203 RR-JF0GF1 RR-JI6F81 RR-JOB6GW RR-JQ25OF
RR-KPQD8A RR-MGV3UP RR-MRJSWS RR-PM7AUW RR-QZ98EZ RR-RL0PUV RR-XX2LE3 RR-Z7J658
RR-ZARZTR

Design-review responses are all addressed. Of the code-review responses, two
nits are wont-fix and field-level write gating is deferred to TKT-0XL8MF, each
with a reason. The size-limit test uncovered an off-by-rounding precheck that
refused a file exactly at the limit; it is fixed and boundary-tested.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: list/read/attach/delete over stdio (manual `rela mcp` run, TestAttachments_StdioAcceptsLargeUpload) and HTTP (TestAttachments_HTTPAcceptsLargeUpload, TestRemoteMCP_HostSharesUploadPolicy).
2. PASS: TestAttachments_ACL_HiddenIsIndistinguishableFromAbsent, TestAttachments_ACL_HiddenPropertyAnswersNotFound.
3. PASS: TestAttachments_ACL_DeniedWriteLeavesBytes (forbidden, bytes unchanged, denied-write audited).
4. PASS: MultiFileSuffixAndCapacity, RejectedUploadIsAudited, UploadTooLarge (boundary), host policy test for the live remote limit.
5. PASS: TestAttachmentContent, ReadReturnsTypedContent, InlineReadCap. Refined in review: images must match their bytes, and non-passive blob types are labeled application/octet-stream.
6. PASS: TestAttachments_ArgumentErrors (bad base64, non-file, undeclared, empty and path file names, non-string file_name).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated (GUIDE-mcp-server, GUIDE-attachment-security; docs regenerated)
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-RTV4DZ

## Final Checks

- [x] ~~Commit message explains the why, not just what~~ (N/A: not committed yet; commits happen on user request)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR is opened on user request, after this checklist per TKT-UFV01M)
