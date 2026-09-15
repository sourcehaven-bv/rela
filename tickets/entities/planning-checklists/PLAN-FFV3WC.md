---
id: PLAN-FFV3WC
type: planning-checklist
title: 'Planning: Compile build-tag-gated test files in CI — they rot silently today'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN:
- A CI guard that compiles (not runs) every `_test.go` file behind a build tag.
- Deciding the fate of the one already-rotted file,
  `internal/dataentry/e2e_test.go`.
- A test for the guard itself.

OUT:
- Running the tagged tests. They need Postgres, a browser, or a human reading
  narrated output; as a pass/fail gate they add nothing over the existing unit
  tests.
- The `maildemo` vet step being added by PR #1566. This guard supersedes it
  (it derives `maildemo` automatically), but that PR is not yet merged and is
  not this ticket's to change.
- GOOS/GOARCH-gated files (`internal/git/clone_windows_test.go`). Those are a
  cross-compilation concern, checked with `GOOS=windows go vet`, not with
  `-tags`. `GOOS=windows go vet ./internal/git/` passes today.

**Acceptance Criteria:**

1. A tagged `_test.go` file that does not compile fails CI.
   Test: `scripts/check-tagged-tests-test.sh` builds a throwaway module whose
   tagged test calls a two-arg function with three args, and asserts the guard
   exits 1.
2. The same tree passes `go build ./...` and untagged `go vet ./...`.
   Test: asserted in the same script, so the guard is demonstrably not
   duplicating a signal that already exists.
3. The tag list is derived from the source, not hardcoded.
   Test: discovery cases for a plain tag, several tags deduped/sorted, and an
   or-expression contributing both terms.
4. Negated constraints and platform tags are not treated as opt-in tags.
   Test: `//go:build !postgres && !memorybackend` and `//go:build windows`
   each yield no tags.
5. An empty match exits 0.
   Test: a module with no tagged files at all exits 0.
6. `e2e_test.go` no longer rots — either compiling and running, or gone.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: single-script CI guard, approach is not in doubt)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — too small to warrant one.

**Existing Solutions:**

- No library needed. `go vet -tags <t> ./...` type-checks without running, which
  is exactly the semantics wanted. `go build` cannot do it: it skips `_test.go`
  entirely.
- Prior art in-repo: `scripts/check-embedded-spa.sh` +
  `scripts/check-embedded-spa-test.sh` establish the convention that a CI guard
  lives in `scripts/check-*.sh` and ships with a companion test asserting the
  NEGATIVE case. Its header states the rule this ticket follows: "A guard that
  has never been observed FAILING is not a verified guard." Both the structure
  and the `want_rc` helper are modelled on it.
- PR #1566 independently diagnosed the same gap and added a narrow
  `go vet -tags maildemo ./internal/mail/ ./internal/appbuild/` step, explicitly
  deferring the rotted `e2e` file to its own ticket. This is that ticket, and
  the derived-tag approach generalises its fix.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`scripts/check-tagged-tests.sh` parses the `//go:build` line of every
`_test.go` file (stopping at the `package` clause, since Go ignores constraints
after it), splits the constraint expression into terms, drops negated terms and
platform tags, and runs `go vet -tags <tag> ./...` for each survivor.

Deriving the list rather than hardcoding it is the whole point: a hardcoded list
is the same class of bug as the one being fixed — it needs a human to remember
to update it. A tag introduced tomorrow is covered with no edit.

`RELA_ROOT` is overridable so the companion test can point the guard at a
throwaway module.

**Alternatives considered:**

- *Hardcode `go vet -tags e2e -tags postgres`*: rejected, needs manual upkeep;
  the next tag rots exactly as `e2e` did.
- *`go test -tags <t> -run XXX ./...`*: compiles and links test binaries, much
  slower, and would try to start Postgres/Chrome on some paths. `vet` is the
  cheaper type-check.
- *One combined `go vet -tags "e2e postgres"`*: wrong. Tags are additive, so
  mutually exclusive tags (`postgres` vs `!postgres` files) would conflict.
  Per-tag invocation matches how the files are actually built.

**Files to modify:**

- `scripts/check-tagged-tests.sh` (new)
- `scripts/check-tagged-tests-test.sh` (new)
- `.github/workflows/ci.yml` (two steps on the existing `demos` job)
- `justfile` (`tagged-tests` recipe)
- `internal/dataentry/e2e_test.go` (deleted — see ticket body)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The only input is the text of `//go:build` lines in the repo's own source,
which is already trusted code — anyone who can edit it can run anything in CI
regardless. It is nonetheless constrained to `^[A-Za-z0-9_.]+$` (an allowlist)
before being interpolated into the `go vet` argument, so a crafted constraint
line cannot inject a shell word or a flag. `find` output is read with
`IFS= read -r`, so paths with spaces are safe.

No credentials, no network, no auth, no crypto. The CI steps take no
`github.event` input, so the workflow-injection class does not apply.

**Security-Sensitive Operations:**

None. The guard only reads source files and runs `go vet`, which does not
execute the code it analyses.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Each acceptance criterion above maps to a named case in
`scripts/check-tagged-tests-test.sh` (14 cases). Integration-level check: the
guard was also run against the real repo tree with the genuinely rotted
`e2e_test.go` restored, and it reproduced the original `NewApp` error.

**Edge Cases:**

- No tagged files at all → exit 0 with a message, not a failure.
- A constraint line after the `package` clause → ignored, as Go ignores it.
- Or/and expressions → every plain term contributes.
- Mixed good and bad tags → still fails, and still vets the remaining tags
  rather than stopping at the first failure.
- `vendor/`, `node_modules/`, `.git/` → pruned; third-party tags are not ours.

**Negative Tests:**

- Stale call signature under a tag → exit 1 (the real-world shape).
- Syntactically invalid tagged file → exit 1.
- One rotted tag among several → exit 1.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *A new tag makes CI slower.* Each tag costs one `go vet ./...`. Three tags
  today; the `demos` job is not on the critical path. Accepted.
- *Discovery misparses an exotic constraint.* Mitigated by the identifier
  allowlist (an unparseable term is dropped, never mis-vetted) and by discovery
  test cases. Worst case is a tag silently not covered, which is the status quo
  ante, not a regression.
- *Conflict with PR #1566, which edits the same job.* Textual only, and the
  overlap is a superset: whichever lands second drops the now-redundant
  `maildemo` step. Noted in the PR body.

**Effort:** s

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified~~ (N/A: internal CI guard, no user-facing surface)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed

The guard documents itself in its own header, the CI step carries a comment
explaining why compiling-not-running is the right call, and `just --list`
surfaces the recipe. No `docs/` page describes the CI job list, so there is
nothing to keep in sync.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None raised. The design question worth recording is
whether to derive tags or hardcode them; deriving won because a hardcoded list
reproduces the very failure mode being fixed. Captured under Approach above.
