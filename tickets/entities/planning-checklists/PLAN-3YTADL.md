---
id: PLAN-3YTADL
type: planning-checklist
title: Planning
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: pure-Go in-process image processor (`internal/imgproc`) for
PNG/JPEG/GIF/WebP-decode; a native `image:` transform step in the metamodel;
decode-config pixel-cap bomb guard; recover()+timeout decode worker; **EXIF
orientation** (Phase 3 folded in — broken rotation is worse than today);
re-encode to PNG/JPEG (metadata strip); wiring in `PolicyProcessor` runnable
with nil `CommandRunner`; CLI parity. OUT: resize/thumbnail + serving (Phase 2),
HEIC/AVIF/JXL/RAW (Phase 4), SVG accept (Phase 4). No change to
scan/ACL/download-hardening/cmdexec.

**Acceptance Criteria:** see the 10 ACs in the ticket body ([[TKT-LNQX4I]]).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** [[RES-WDFS96]]

**Existing Solutions:**
- stdlib `image/{png,jpeg,gif}` + `golang.org/x/image/{webp,draw}` — pure-Go, memory-safe, already partly linked (`image/jpeg` linked today; `x/image` v0.40.0 already in the module graph). Measured binary delta of the full stack: **+417 KiB (<1%)**.
- `disintegration/imaging` v1.6.2 (2019-frozen, pure-Go) — provides `AutoOrientation`; candidate for orientation, weighed against a small inline EXIF-orientation reader (Open Q1).
- Existing pipeline seam: `attachment.Processor` (`internal/attachment/processor.go:30`), `PolicyProcessor` (`policy.go`), `TransformStep` (`internal/metamodel/types.go:320`). This ticket extends FEAT-KTZJIV's `cmd:` pipeline with a native step kind.
- Prior art contrast: the bwrap `cmdexec` work (PR #1188, [[RES-WDFS96]] Option A) contains the *external* tools; this ticket removes the need for them on common formats.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** see Design in [[TKT-LNQX4I]]. Native `image:` step runs
`imgproc.Normalize` in-process (no runner, no sandbox); `cmd:` steps keep the
sandboxed path; mixed pipelines run in declared order.

Alternatives rejected: (a) WASM Rust image module ([[RES-WDFS96]] Option B) —
more cost, only justified for SVG; (b) always shell to vips under bwrap (Option
A) — external-tool + sandbox burden for the 95% case; memory-safe Go removes
both.

Dependencies: stdlib `image/*`, `golang.org/x/image/{webp,draw}`; possibly
`disintegration/imaging` (Open Q1). Bump `x/image` to ≥ v0.43.0 (patched).

**Files to modify:**
- NEW `internal/imgproc/` (Normalize, Config, errors, tests, fuzz)
- `internal/metamodel/types.go` (`TransformStep.Image *ImageStep`), loader validation
- `internal/metamodel/attachments.go` (global image defaults, optional)
- `internal/attachment/policy.go` + `command.go` (run native steps; nil-runner path)
- `internal/dataentry/app.go` (config plumbing), `internal/cli/cli_wiring.go` (CLI parity)
- docs: `docs/attachment-security.md` / metamodel docs

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Attacker-controlled upload bytes → `io.LimitReader` cap → `DecodeConfig` pixel-cap (reject before alloc) → decode in recover()+timeout worker. Invalid → 422 wrapping `ErrRejected`, generic message (no internal detail leak).
- Metamodel `image:` config is operator-trusted, but still allowlist-validated at load (known `reencode` values; mutually-exclusive with `cmd:`).

**Security-Sensitive Operations:**
- No file access beyond the runner-owned bytes; no shell, no external process, no network — the whole point vs `cmd:`. Threat reduced to DoS-only (memory-safe Go). DoS bounded by pixel-cap + timeout + concurrency semaphore + `GOMEMLIMIT` backstop. Residual: keep `x/image` patched (govulncheck per-PR covers the continuous CVE stream).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** each AC → test as listed in the ticket Test plan. `imgproc`
unit+fuzz; metamodel loader validation; `internal/attachment` pipeline
integration (native-only/cmd-only/mixed/nil-runner); CLI `rela attach` image
step.

**Edge Cases:** all 8 EXIF orientations; truncated/corrupt streams; palette-OOB;
oversize-declared-dims (bomb); WebP→PNG/JPEG re-encode; zero-byte; animated GIF
(first frame policy — decide); already-canonical no-op.

**Negative Tests:** dims over cap → 422 no large alloc; undecodable + required
step → 422; malformed metamodel step (both/neither/bad
reencode/image-on-non-file) → load error; crafted panic corpus → handled error,
`-race` green, process survives.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Animated GIF / multi-frame: re-encode to a single-frame format loses animation — must be a *documented, intentional* behaviour, not a silent surprise (decide in review).
- `x/image` CVE cadence: mitigated by govulncheck + version bump; documented as an operational requirement.
- `imaging` is frozen (2019): acceptable if used only for orientation; the inline-reader alternative removes the risk entirely (Open Q1).
- Effort: **m**.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/metamodel.md — new native `image:` transform step
- [x] docs/attachment-security.md — native normalisation/strip alongside `cmd:` recipes
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting convention; the feature follows the existing Processor/transform patterns)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 7 findings (6 significant, 1 minor), all addressed:
[[RR-7R3EYR]] (filename extension on re-encode), [[RR-4G5YBU]] (output type safe
by construction), [[RR-S8FEUJ]] (semaphore is a required control), [[RR-ZM3PE7]]
(decode goroutine bounded, not cancellable), [[RR-01E7DG]] (reject animated GIF),
[[RR-U517I0]] (orientation default / likely always-on), [[RR-9IVLFO]] (coverage
floor). Resolutions folded into the ticket's "Design review outcomes" section.
No critical findings; no design blockers remain.
