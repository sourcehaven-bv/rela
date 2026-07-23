---
id: PLAN-0ZQHXY
type: planning-checklist
title: 'Planning: Split docs build into a separate rela-docs binary (unlink chromedp from rela)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: Move `docs build` out of the `rela` CLI into a new `cmd/rela-docs` binary so
chromedp (pulled in by `screenshot{}` via `internal/docscapture`) is no longer
linked into `rela`/`rela-server`. Extract the command + capturer seam into a new
`internal/docscli` package behind a narrow consumer-side `Project` interface.
Wire justfile/goreleaser/CI/arch-lint; add a CI link-isolation assertion; update
docs. Breaking change: `rela docs build` → `rela-docs build`.

OUT: Any change to the doc language itself, the resolvers, or the capture
mechanics. No new screenshot features. No behavior change to the rendered
output.

**Acceptance Criteria:**
1. `go list -deps ./cmd/rela | grep chromedp` empty; same for `rela-server`;
present for `rela-docs`. → CI "Assert dependency isolation" step.
2. `go build ./cmd/rela` back to ~47 MB (was 62). → measured: 46.8 MB.
3. `rela-docs build <manual>` renders identically, incl. `screenshot{}`. →
verified live against the tickets project + docscli tests.
4. All build-tag combos compile; postgres rela-docs links no chromedp. → CI.

## Research

- [x] ~~Run /research~~ (N/A: mechanical extraction, approach obvious)
- [x] Checked codebase for similar patterns

**Existing Solutions / prior art in-tree:**
- The postgres/fs build-tag backend split (`appbuild_{fs,postgres}.go`,
`docs_capturer_{fs,postgres}.go`) is the exact pattern — a tag-selected edge
file is the sole importer of the heavy dep, CI asserts isolation with `go list
-deps | grep`. This change adds an analogous chromedp invariant.
- `cmd/rela` ↔ `internal/cli` split: the binary is a thin main; the testable
command layer is a package. Mirrored here with `cmd/rela-docs` ↔
`internal/docscli`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** New `internal/docscli` owns `BuildCmd.Run` (moved from
`internal/cli/docs.go`) and the build-tagged `NewCapturer` seam. It depends on a
consumer-side `Project` interface (`Meta()`, `Paths()`) that `appbuild.Services`
satisfies structurally — replacing the old 13-field `*readServices` bundle of
which only 2 fields were used. `cmd/rela-docs/main.go` is a small Kong root that
runs `appbuild.Discover` and binds the interface. `internal/cli` drops the
`Docs` field and stops importing docs/docscapture entirely — that is what severs
chromedp from `rela`.

**Alternatives considered:**
- Build-tag gate in `rela` (keep one binary): rejected — user wants a separate
tool; a tag axis wouldn't slim the default binary without also removing the
subcommand.
- Command directly in `cmd/rela-docs` (no docscli package): rejected — a main
can't be unit-tested; the package enables the docscli_test suite.

**Files modified:** internal/docscli/{docscli,capturer_fs,capturer_postgres}.go
(new), cmd/rela-docs/main.go (new), internal/cli/kong.go, internal/errors/
errors.go (shared WrapDiscoverError), deleted internal/cli/docs*.go, justfile,
.goreleaser.yaml, .github/workflows/ci.yml, .go-arch-lint.yml, .gitignore, docs.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new inputs vs. the old `rela docs build`. The
manual path is operator-supplied (same trust boundary as `pandoc in.md`);
acl.yaml is read from the project root. The postgres build refuses
`screenshot{}` (would seed the live DB) — a fail-loud safety property, not a new
attack surface.

## Test Plan

- [x] Test scenarios documented per acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** docscli_test.go covers stdout render, `--out` file render
(creates parent dirs), acl.yaml present→roles_matrix renders, acl.yaml
malformed→fail-loud, missing manual→error, `--out` is a dir→error, screenshot{}
without browser→fail-loud (via injectable `newCapturer` seam), outputDir units.
CI asserts the dependency isolation directly.

**Edge Cases:** bare output filename (`filepath.Dir` → "."), absent acl.yaml
(degrades), Chrome absent (fail-loud, deterministic via seam).

## Risk Assessment

- [x] Technical risks assessed
- [x] Security risks assessed
- [x] Effort estimated (m)

**Risks:** Breaking CLI change (`rela docs` removed) — mitigated by docs update
and the ticket noting it. Isolation regressing later — mitigated by the CI
assertion (both directions + postgres). Cross-compile of the frontend-embedding
binary — verified bare builds on linux/darwin/windows.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] GUIDE-rela-docs.md (`rela docs build` → `rela-docs build`, build-docs, split rationale)
- [x] docs/rela-docs.md regenerated via `just docs`
- [x] prototypes/data-entry/manual/tickets-manual.md reference updated

## Design Review

- [x] ~~Run /design-review~~ (N/A: mechanical extraction; reviewed post-impl by cranky + go-architect instead)
- [x] All critical/significant findings addressed

**Review Findings:** RR-CWEDAQ, RR-V9KEES, RR-0AJ0PI, RR-1NTHFT (all addressed).
