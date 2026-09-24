---
id: REV-IKYFO1
type: review-checklist
title: 'Review: Remote MCP returns 403 behind proxy (go-sdk localhost guard)'
status: done
---

## Automated Checks

- [x] All tests pass (`go test ./internal/mcp ./internal/dataentry ./cmd/rela-server`)
- [x] Lint clean (`golangci-lint run` on the changed packages: 0 issues)
- [x] Coverage maintained (new test covers the changed line)

`TestHTTPHandlerAcceptsProxiedHost` fails with 403 before the fix and passes after.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-IKY001 (significant, addressed), RR-IKY002
(significant, wont-fix with reasoning), RR-IKY003 (minor, addressed),
RR-IKY004 (minor, addressed). The reviewer verified the security argument: the
route sits behind `requireVerifiedJWT` with no exemption, and `SetRemoteMCP`
refuses without a JWT gate.

## Acceptance Verification

- [x] Remote MCP `initialize` with a public Host over a loopback connection returns 200

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use
