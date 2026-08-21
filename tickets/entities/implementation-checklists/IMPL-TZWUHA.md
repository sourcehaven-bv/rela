---
id: IMPL-TZWUHA
type: implementation-checklist
title: 'Implementation: Adopt commentlint in CI: comment-discipline gate + advisory report'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no Go code is added to this
repo. The linter's rules are unit-tested upstream — 20 tests in
sourcehaven-bv/commentlint covering config parsing, glob matching, inline
directives, and each rule's behaviour including its known false-positive
classes. What this ticket adds here is CI wiring and config, verified by running
it.)
- [x] ~~Integration tests written~~ (N/A: the integration IS the CI job;
verified by executing every recipe and parsing the workflow — see Manual
Verification.)
- [x] Happy path implemented — gate + advisory report both run
- [x] Edge cases from planning handled — UTF-8 truncation fixed upstream
before adoption; absent-config and narrow-suppression cases covered by upstream
tests
- [x] Error handling in place — a malformed `.commentlint.yml` fails with a
line number rather than being silently ignored

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests
added in this repo)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Every gate run with its exit code captured:

```
lint         EXIT=0
arch-lint    EXIT=0
plimsoll     EXIT=0
comment-lint EXIT=0   ("no findings across 9876 comments")
lint-md      EXIT=0
```

`just ci` was also run to completion twice on this branch: `0 issues` from
golangci-lint, all package tests `ok`, `Summary: 0 issues in 0 files` from
markdownlint, and `✓ Docs are up to date.` Zero `FAIL` lines across the run.

Per acceptance criterion:

1. **Gate exits 0 on this branch** — `just comment-lint` → EXIT=0, "no findings
across 9876 comments".
2. **Gate catches a regression** — `commented-code` is 0 today; the rule is
unit-tested upstream. Not re-verified by planting commented-out code here, since
that would mean committing a deliberate defect.
3. **Advisory report runs and never fails** — `just comment-report
param-contract` lists 5 findings and exits 0.
4. **CI job registers** — `ci.yml` parsed with a YAML loader; `comment-lint`
present with 5 steps, alongside the 15 existing jobs.
5. **Suppression works both ways** — the inline directive is used in anger at
`internal/imgproc/orientation.go:48` (the `exifHeaderLen` false positive, where
the literal `"Exif\0\0"` IS the reason for the value), and `allow-phrases`
carries the ACL nil-vs-empty-slice idiom. Both verified end-to-end upstream
before release.

## Quality

- [x] Code follows project patterns — deliberately mirrors the plimsoll job
(`ci.yml:153`) and its justfile recipe, including the pinned version and the
"keep in sync" comment on both sides
- [x] Checked for DRY opportunities — the version string is duplicated between
`justfile` and `ci.yml`. Left duplicated on purpose: that is the existing
convention for `plimsoll_version`, and a shared source would need a new
mechanism for one string. Both sites carry a sync comment.
- [x] No security issues introduced — CI installs one more pinned third-party
binary from an org-owned repo through the module proxy, the same exposure
plimsoll already carries. One security *improvement*: `credentialFileMode` now
documents why `0600` matters (the file holds an access token in cleartext)
instead of restating the constant.
- [x] No silent failures — the gate fails the build; the advisory step is
explicitly `continue-on-error` and says so in a comment
- [x] No debug code left behind

**Comment fixes included (found by the tool, verified by hand):**

| File | Change |
|---|---|
| `dataentryconfig/validate.go` | 4 pure restatements deleted (siblings already had none) |
| `lua/markdown.go`, `schema/analyze.go`, `fsstore.go`, `importer.go`, `affordances/resolver.go`, `dataentry/caldav_ctag.go` | restatements on unexported helpers deleted |
| `git/clone.go` | `credentialFileMode` — documents why `0600` (cleartext token) |
| `lua/runtime.go` | `hoursPerDay` — elapsed-time vs calendar-day, DST caveat |
| `imgproc/orientation.go` | inline suppression of a false positive, with reason |

Exported-decl restatements were deliberately NOT "fixed": golangci-lint enables
revive's `exported` rule, so every exported symbol must carry a doc starting
with its name. For a trivial accessor the name genuinely says it all, and the
rule's advice ("otherwise delete") would break the build. Those 19 stay advisory
— a limitation of the rule on exported Go API, not a backlog.
