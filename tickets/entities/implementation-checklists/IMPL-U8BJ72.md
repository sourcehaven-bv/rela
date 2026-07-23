---
id: IMPL-U8BJ72
type: implementation-checklist
title: 'Implementation: Split docs build into a separate rela-docs binary (unlink chromedp from rela)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (internal/docscli/docscli_test.go, 93.3%; internal/errors WrapDiscoverError test, 100%)
- [x] Integration tests written (docscli_test drives BuildCmd.Run end-to-end against a fake Project + temp manuals/acl.yaml)
- [x] Happy path implemented (stdout + --out render)
- [x] Edge cases from planning handled (bare filename, absent/malformed acl.yaml, no browser)
- [x] Error handling in place (fail-loud on missing manual, malformed acl, screenshot without browser, --out is a dir)

## Test Quality

- [x] Using fixture builders (newFakeProject, writeManual, stub[No]Capturer helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use rendered output / error, not brittle strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**
- `rela-docs build mini-manual.md --project tickets` → rendered field table
(typeref), roles_matrix no-policy note, description() echo. ✓
- `rela` no longer lists a `docs` subcommand; `rela docs build` → usage error. ✓
- `rela-docs --version` (with `-X main.Version=1.2.3-test`) → `1.2.3-test`. ✓
- `go list -deps ./cmd/rela | grep -c chromedp` → 0; `./cmd/rela-server` → 0;
`./cmd/rela-docs` → 62; `-tags postgres ./cmd/rela-docs` → 0. ✓
- Binary size: `go build ./cmd/rela` = 46.8 MB (was 62). ✓
- All build tags compile (default, memorybackend, postgres). ✓

## Quality

- [x] Code follows project patterns (consumer-side interface; build-tag seam mirrors appbuild)
- [x] Checked for DRY opportunities (extracted shared WrapDiscoverError; left principal/Discover wiring duplicated per "a little copying beats a little dependency")
- [x] No security issues introduced (no new inputs; postgres screenshot refusal preserved)
- [x] No silent failures (all error paths surfaced and returned)
- [x] No debug code left behind

**Quality gates:** `just arch-lint` clean · `just plimsoll` PASS · `go vet
./...` clean · golangci-lint 0 issues · `just coverage-check` PASS (total
76.4%).
