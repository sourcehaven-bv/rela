---
id: IMPL-B7XCCT
type: implementation-checklist
title: 'Implementation: Compile build-tag-gated test files in CI — they rot silently today'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`scripts/check-tagged-tests-test.sh` covers the guard at unit level (30 cases
over throwaway modules). The integration-level check is the guard run against
the real repo tree, both with and without the genuinely rotted `e2e_test.go`.

Review caught the guard reproducing its own failure mode. The first draft
parsed constraints with `sed`/`grep` and got three things wrong: a TAB after
`//go:build` was not matched at all (so a rotted file gave "nothing to compile"
and exit 0); the legacy `// +build` syntax was ignored though Go still honours
it; and `!(a || b)` yielded both tags because stripping parens orphans the `!`
from its group. Discovery moved to `scripts/tagged_build_tags.go`, which uses
Go's own `go/build/constraint` parser and is correct by construction.

The loop deliberately does not stop at the first failing tag: it records
`fail=1` and continues, so one broken tag does not mask another.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`make_module` and `add_test` are the fixture builders; each case specifies only
the build line and body that matter to it. `want_rc` / `want_tags` are the two
assertion helpers, following `scripts/check-embedded-spa-test.sh`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. Guard on the clean tree — passes, and discovers the tags itself:

```
$ ./scripts/check-tagged-tests.sh
Build tags found on Go files: maildemo mailmanual memorybackend postgres sqlite
==> go vet -tags maildemo ./...
==> go vet -tags mailmanual ./...
==> go vet -tags memorybackend ./...
==> go vet -tags postgres ./...
==> go vet -tags sqlite ./...
OK: all build-tag-gated files compile.   # exit 0
```

2. Guard against the REAL regression. Restoring the rotted `e2e_test.go`
   (`git show HEAD:internal/dataentry/e2e_test.go > ...`) makes the guard pick
   up the `e2e` tag on its own and fail with the original error:

```
Build tags found on Go files: e2e maildemo mailmanual memorybackend postgres sqlite
==> go vet -tags e2e ./...
vet: internal/dataentry/e2e_test.go:62:2: not enough arguments in call to NewApp
ERROR: tagged files do not compile under -tags e2e.
   # exit 1, and the remaining tags are still vetted
```

3. Guard test suite — 30/30, including the two cases that prove the guard is
   not redundant (on the same rotted tree, `go build ./...` and untagged
   `go vet ./...` both exit 0 while the guard exits 1):

```
$ ./scripts/check-tagged-tests-test.sh
passed: 30  failed: 0   # exit 0
```

4. Platform tags are correctly excluded rather than mis-vetted. `go vet -tags
   windows ./...` produces a spurious GOOS-redeclared error on darwin; the
   right check is `GOOS=windows go vet ./internal/git/`, which passes today.

5. `just tagged-tests` runs the guard; `just --list` shows it.

6. `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"`
   parses, so the workflow edit is well-formed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns: `scripts/check-*.sh` plus a companion `-test.sh`, mirroring
`check-embedded-spa.sh`. `want_rc` is reused from that file's design rather
than reinvented.

DRY: `is_platform_tag` and `discover_tags` are separate because they answer
different questions (is this tag ours vs. which tags exist). The per-case
`make_module` calls in the test are intentionally repeated — each case needs
its own tree, and folding them into a table would obscure what each asserts.

Security: the only interpolated value is a build tag constrained to
`^[A-Za-z0-9_.]+$` before reaching `go vet`. `shellcheck` is clean apart from
one SC2001 style note on a line copied verbatim from the existing
`check-embedded-spa-test.sh`, kept for consistency.

No silent failures: a failing tag sets `fail=1`, prints the offending command,
and exits 1 with an explanation of why no other CI step would have caught it.
The one non-failing path — no tagged files at all — prints why it is a no-op
rather than exiting silently.
