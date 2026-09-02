---
id: PLAN-4K4H9E
type: planning-checklist
title: 'Planning: Require BaseDir on git.Clone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `containedPath` rejects an empty `base` instead of skipping containment, and
`CloneOptions.BaseDir` is documented as required.

OUT, with reasons:

- *Symlink resolution.* `containedPath` is deliberately string-level (Clean +
Rel), matching `storage.RootedFS`'s threat model: the target is "the
caller-supplied final segment contains traversal syntax", not "an attacker
already has write access to the base directory". Widening that is a different
ticket with a different argument, and folding it in here would make this change
hard to reason about.
- *Changing `ExtractRepoName` to refuse `..`.* It already has its own test
(`TestExtractRepoName_RejectsTraversal`). Defence at the `Clone` boundary is the
point of this ticket — the boundary must hold even when a caller's own
sanitisation is absent or wrong.

**Acceptance Criteria:**

1. `Clone` with an empty `BaseDir` fails with a required-BaseDir error.
*Test:* a path that would be perfectly valid under a real base, so the failure
is attributable to the missing base and nothing else.
2. The traversal shape from the issue fails on the MISSING BASE specifically.
*Test:* `Path = <tmp>/..` with no `BaseDir`; assert the error names the required
base, not merely that some error occurred. A bare non-nil check would pass on a
network error or "path already exists" and leave the traversal unguarded on a
machine where those happen not to fire.
3. Existing containment behaviour is unchanged.
*Test:* the pre-existing `TestClone_RejectsPathEscapingBaseDir` and
`TestClone_AllowsPathInsideBaseDir` still pass unmodified.
4. A base that cleans to the filesystem root is REFUSED.
*Test:* `containedPath("/", "/etc/passwd")` returns an error naming the root.
**ADDED AFTER REVIEW** (RR-Q5N3CJ) — a root base contains everything, so the
check passed while verifying nothing.
5. The `os.Stat` path-exists guard keeps its coverage.
*Test:* `TestClone_PathExists` supplies a `BaseDir` so it reaches that guard,
and asserts on the error text. **ADDED AFTER REVIEW** (RR-BPGG9C) — making
`BaseDir` required had moved that test's failure point to the new guard, leaving
`os.Stat` uncovered while the test still passed.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a one-branch change.

**Existing Solutions:**

- `storage.RootedFS` is the in-repo prior art for path containment and is
already cited by `containedPath`'s comment; this ticket does not change the
model, only closes the "no base supplied" hole in it.
- No library needed: `filepath.Clean`/`Rel` already do the work.
- Checked every caller of `Clone`: there is exactly one
(`cmd/rela-desktop/main.go:364`) and it already sets `BaseDir`. So making the
field required breaks nothing today — which is precisely why it should be done
now rather than after a second caller exists.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Move the empty-base branch from the middle of `containedPath` (where it returned
the path unchecked) to the top, where it returns an error. Update the
`CloneOptions.BaseDir` doc from "when non-empty" to "REQUIRED", and record in
both comments *why* it is required, since a future reader's instinct will be
that an optional field is friendlier.

**Alternatives considered:**

- *Default `BaseDir` to the process CWD when empty.* Rejected: it would contain
the clone somewhere the caller never named. That is a different surprise, not a
smaller one, and it would make the containment guarantee depend on the working
directory of whatever process happened to call.
- *Leave it optional and fix the doc instead* (i.e. admit containment is
caller-supplied). Rejected: it gives up the property the field exists for. The
value of a boundary check is that it holds when the caller is wrong.
- *Validate in `Clone` before calling `containedPath`.* Rejected as strictly
worse: `containedPath` is the function that owns the invariant, so a caller
reaching it directly (a future test, a future call site) should get the same
answer. Putting the guard in the callee means there is one place to be right.

**Files to modify:**

- `internal/git/clone.go`
- `internal/git/clone_test.go`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `Path` is caller-supplied and its final segment can derive from a
remote-controlled value: `ExtractRepoName` returns the URL's last path segment
and can yield `..`.
- `BaseDir` is operator/caller-supplied and now mandatory.
- Validation is containment against an explicit base (an allowlist of one
subtree), not a blocklist of bad substrings — `filepath.Rel` on cleaned absolute
paths, so `..`, absolute paths and mixed forms all reduce to the same question.

**Security-Sensitive Operations:**

`storeCredentials` writes a plaintext OAuth token to `<Path>/.git/credentials`.
That is what makes a traversal here a credential disclosure and not merely a
misplaced directory, and it is why the check belongs at this boundary.

The error message names the base directory and the path — both already known to
the caller who supplied them, so no disclosure.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one per acceptance criterion, driven through the exported
`Clone` rather than `containedPath` directly — the defect is that the BOUNDARY
fails open, so the test has to cross the boundary.

**Edge Cases:**

- empty `BaseDir` with an otherwise valid path → rejected (AC1).
- empty `BaseDir` with a traversal path → rejected on the missing base (AC2).
- `BaseDir` set, path inside → allowed, unchanged.
- `BaseDir` set, path escaping / equal to base / unrelated absolute → rejected,
unchanged (the existing table test covers all four).

**Negative Tests:**

AC2 is the load-bearing one, and specifically its assertion on the error TEXT.
`Clone` fails for many reasons in a test environment (no network, not a real
repo), so asserting only `err != nil` would pass against the unfixed code by
accident.

Mutation check: restore the `if base == "" { return abs, nil }` fail-open and
confirm exactly the two new tests redden and nothing else.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Tightening a precondition relocates existing tests' failure points.* MISSED
in planning, found in review (RR-BPGG9C). `TestClone_PathExists` constructed
`CloneOptions` without a base, so it began failing at the new guard, never
reached the `os.Stat` check it exists to cover -- and kept PASSING, because its
assertion was a bare `err == nil`. That guard had zero coverage while the suite
was green.

The plan asked "does this break an existing caller" and checked production code.
It never asked the same question of the TESTS, which are also callers, and which
fail silently in the one direction that looks like success.

- *The check being correct is not the same as the change being safe.* Three
further findings (RR-HLWRMK, RR-L3CC5O, RR-Q5N3CJ) were all adjacent to a
containment check that was itself correct: a value cached before validation that
later reached MkdirAll, a dropped error yielding a relative base, and a root
base that passed while checking nothing. Planning verified the guard and never
asked what surrounded it.

- *Breaking an existing caller.* Checked: one caller, already compliant. The
change is source-compatible (no signature change), so a future caller that
forgets gets a clear runtime error naming the missing field rather than silent
unsafety — which is the entire point.
- *Someone re-optionalises it later for convenience.* Mitigated by putting the
reasoning in the doc comment on the field AND on `containedPath`, at both places
a person would edit.

**Effort:** xs

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — `internal/git` is not a public API and `Clone` has one in-tree
caller that already complies. Nothing an operator configures or invokes changes.
The documentation that matters here is the godoc, which is where the "why
required" reasoning goes.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none critical or significant. The one decision worth
recording is rejecting a CWD default — an "obvious" convenience that would
silently relocate the containment boundary rather than enforce it.
