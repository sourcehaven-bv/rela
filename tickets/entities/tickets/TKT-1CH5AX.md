---
id: TKT-1CH5AX
type: ticket
title: Compile build-tag-gated test files in CI — they rot silently today
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

Build-tag-gated `_test.go` files are compiled by nothing in CI, so they rot
silently.

`go build ./...` does not compile `_test.go` files at all, and `go test ./...`
only compiles the files whose build constraints the default tag set satisfies. A
file behind `//go:build e2e` is therefore invisible to every CI step. It can
reference a function signature that changed releases ago and nothing says a
word.

That is not hypothetical. On develop @ b34dbae8:

```
$ go vet -tags e2e ./internal/dataentry/
vet: internal/dataentry/e2e_test.go:62:2: not enough arguments in call to NewApp
	have (..., store.Store, *entitymanager.Manager, search.Searcher, acl.ACL, NopFieldVerdictResolver, audit.Audit)
	want (..., store.Store, store.VersionService, *entitymanager.Manager, search.Searcher, search.VisibleSearcher, acl.ACL, FieldVerdictResolver, audit.Audit, state.KV)
```

`internal/dataentry/e2e_test.go` has been untouched since #817 while
`dataentry.NewApp` gained `store.VersionService`, `search.VisibleSearcher` and
`state.KV`. Nothing caught it. PR #1566 noticed the same gap while adding a
`maildemo` demo, added a narrow `go vet -tags maildemo` step for its own files,
and explicitly left this to its own ticket.

## Scope

Every build tag on a Go file, surveyed repo-wide. The guard covers production
files as well as tests: `//go:build sqlite` code is skipped by the default build
for the same reason a tagged test is, so scoping to `_test.go` would have left
half the rot uncovered.

| tag | files | state |
|-----|-------|-------|
| `postgres` | 11 tests + production | compiles |
| `maildemo` | 1 test | compiles |
| `mailmanual` | 1 test | compiles |
| `sqlite` | production only | compiles |
| `memorybackend` | production only | compiles |
| `e2e` | 1 test | **does not compile** |
| `windows` | 1 test | compiles (GOOS, not an opt-in tag) |

Negated constraints (`//go:build !postgres && !memorybackend`) mark files that
build by default and are already covered by the normal test run.

## Fate of internal/dataentry/e2e_test.go

Deleted, not repaired. Its one test, `TestE2E_LuaDocumentRenders`, drove
headless Chrome via chromedp to assert three things about a Lua-rendered
document. All three are covered today by tests that actually run:

1. **Lua document renders in the SPA** — `e2e/tests/back-button.spec.ts`
renders the Lua-backed `feature-overview` document (fixture script
`scripts/docs/feature_overview.lua`) in a real browser and asserts on the
rendered body. Playwright runs in CI; chromedp did not.
2. **`rela.url` produces a working link** — the same spec clicks the
`rela.url.detail` link inside the rendered body and asserts the resulting
navigation.
3. **The link rewriter appends `return_to` on form routes** —
`internal/dataentry/document_test.go` covers the form-route branch exhaustively
as untagged unit tests: exact hrefs, the stable `id=` scroll anchor, query-param
preservation, and author-planted `return_to` spoofing. Those run on every `go
test`.

Repairing it would mean maintaining a second browser automation stack (chromedp
alongside Playwright) for coverage that already exists, and it would depend on
`prototypes/data-entry/project` plus a local Chrome. It would also still not run
in CI, which just re-arms the same trap. Deleting it is the honest outcome.
`chromedp` remains a direct dependency via `internal/docscapture`, so `go.mod`
is unchanged.

This is a deliberate narrowing, not a lossless move. Each of the three
assertions is covered, but one *seam* is not covered by any single test: that a
URL built inside the Lua VM survives goldmark and reaches the rewriter in the
right shape is now implied by two tests either side of it rather than asserted
end to end. The deleted test did not defend that seam either — it had not
compiled for months, and it `t.Skip`s when the prototype fixture is absent — so
nothing regresses today. Closing it properly means adding a `rela.url.form_edit`
link to the Playwright fixture document (`e2e/tests/fixtures.ts`) and asserting
the rewritten href; that is a Playwright change, not a reason to keep dead Go
code.

## Fix

`scripts/check-tagged-tests.sh` vets every build tag that appears on a Go file,
wired into the existing `demos` CI job and available as `just tagged-tests`.

The tag list is **derived from the source**, not hardcoded, so a tag introduced
tomorrow is covered without anyone remembering to edit the guard. The script
skips GOOS/GOARCH tags (`go vet -tags windows` type-checks against the host
platform and collides with the real GOOS) and ignores negation-only constraints.
An empty match exits 0, so deleting the last `e2e` file does not break the
build.

These tests are compiled, not run: they need Postgres, a browser, or a human
reading narrated output, so as a pass/fail gate running them adds nothing over
the existing unit tests.

## Verification

`scripts/check-tagged-tests-test.sh` follows the repo convention that a guard
which has never been observed failing is not a verified guard. It asserts the
negative cases against a throwaway module, including that on a tree with a
rotted tagged file `go build ./...` and untagged `go vet ./...` both exit 0
while the guard exits 1 — proving the guard is not duplicating a signal that
already exists. 19/19 pass.

Review caught the guard reproducing its own failure mode. The first draft parsed
constraints with `sed`/`grep`, which got three things wrong:

- `//go:build<TAB>e2e` was not matched at all, so a rotted file produced
"nothing to compile" and exit 0 — precisely the silent pass this guard exists to
prevent, inside the guard itself.
- The legacy `// +build` syntax was ignored, though Go still honours it when no
`//go:build` line is present.
- `!(alpha || beta)` yielded `alpha` and `beta`, because stripping parens
textually orphans the `!` from the group it negates. That file builds by
default, and `-tags alpha` would have EXCLUDED it, so the guard was claiming
coverage it did not have.

Patching the patterns would have left the next gap, so discovery moved to
`scripts/tagged_build_tags.go`, which uses Go's own `go/build/constraint` parser
and is correct by construction for whitespace, parens, negation scope, both
syntaxes and their precedence. The test suite grew from 14 to 30 cases,
adversarial ones included.

Checked against the real regression too: restoring the rotted `e2e_test.go`
makes the guard auto-discover the `e2e` tag and fail with the original `NewApp`
error; with it deleted the guard is green.
