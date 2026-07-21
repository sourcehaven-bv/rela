---
id: REV-RKAA11
type: review-checklist
title: 'Review: JWT identity must fail closed, never downgrade to --principal-header'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated checks

- [x] `go test ./...` — full suite passes
- [x] `go test -race` on the three affected packages — clean (the gate holds shared sampling state)
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — clean
- [x] `just plimsoll` — clean
- [x] `just coverage-check` — PASS (76.3%)

## Code review

- [x] `/code-review` run via cranky-code-reviewer on the full diff, framed as a security change
- [x] All 5 findings triaged into review-response entities (2 significant, 2 minor, 1 nit)
- [x] Both significant findings fixed ([[RR-2LPY46]], [[RR-39YRW3]])
- [x] Both minor findings fixed ([[RR-LHFXGZ]], [[RR-QE3XJG]])
- [x] Nit recorded as wont-fix with reasoning on file ([[RR-W5TQNB]])
- [x] No open critical or significant responses

Reviewer independently confirmed the two load-bearing design decisions rather
than taking the comments at face value: gate placement (stamper → gate → ACL,
verified against `attachACLRequest` reading `principal.From` at router.go:258)
and `/api/`-only scoping (checked the full route table, including that
`/api/sync/*` is covered despite being same-origin-exempt, and that SSE on the
outer mux is still gated). Also probed path-normalization bypasses (`//api/...`,
`/foo/../api/...`, `%2e%2e`) — `ServeMux` 307-redirects to the cleaned path
before the handler runs, so those fail closed.

## Verification

- [x] Startup refusal verified against the real binary for each conflicting config
- [x] Runtime paths verified live against a real ES256 JWKS server: 200 valid /
401 absent, expired, forged-signature, malformed, spoofed header, bare `/api`,
SSE / 200 SPA shell
- [x] **Cached-keys claim verified empirically** — with the JWKS server killed, a
valid assertion still returned 200, confirming the ticket's open question
- [x] Header-only mode re-verified unchanged (still fails open)
- [x] Log sampling + recovery reporting verified live (`suppressed_invalid=7`)
- [x] All runtime paths re-verified after the post-review refactor

## Follow-up

- [x] Pre-existing unreachable-webhook bug filed separately as [[BUG-F3ADZO]]
with a preventive measure, rather than folded into this change
