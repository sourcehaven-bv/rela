---
id: DOCS-86O9DW
type: docs-checklist
title: 'Documentation: Compile build-tag-gated test files in CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The non-obvious parts are all documented at the point of the decision:

- `scripts/tagged_build_tags.go` opens with why discovery is a Go program and
  not a grep — the first draft's sed parser missed `//go:build<TAB>e2e` and let
  a rotted tree pass. `collect()` documents why negation POLARITY matters (a
  file behind `!postgres` builds by default, so `postgres` is not an opt-in
  tag), and `fileTags` documents the `//go:build` over `// +build` precedence
  rule. The known benign false positive (a constraint inside a block comment)
  is recorded rather than left for someone to rediscover.
- `platformTags` explains why GOOS/GOARCH tags are skipped: `go vet -tags
  windows` type-checks against the HOST and collides with the real GOOS.
- `scripts/check-tagged-tests.sh` explains why compiling is the right check
  rather than running, and why each tag is vetted separately.
- The CI steps say why the self-test runs first, and why `go build ./...`
  cannot substitute.

No public Go API is added: the tool is `//go:build ignore`, so it is not part
of the package graph.

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing feature)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing
  `scripts/check-*.sh` + companion `-test.sh` pattern set by
  `check-embedded-spa.sh`; no new convention introduced)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

`just --list` documents both new recipes (`tagged-tests`,
`tagged-tests-test`), and the guard has a usage line for its one argument.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo has no CHANGELOG; releases are cut
  from commit history)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API surface changed)

The ticket itself carries the reasoning a future reader needs: why
`e2e_test.go` was deleted rather than repaired, and which seam that narrows.
