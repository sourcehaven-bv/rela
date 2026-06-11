---
id: IMPL-22LOE7
type: implementation-checklist
title: 'Implementation: Default package coverage floor so new untested packages fail CI'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — internal/config gained its first test file (92.6% coverage; round-trip, os.IsNotExist contract, 10-case unsafe-name table, Subscribe validation) per review finding RR-R375DS
- [x] Integration tests — n/a (config change)
- [x] Happy path implemented (default floor 50; overrides git 30, cli 30, search 20, mcp 50, config 87; exclusions entitymanagertest + graphquerynaive with rationale comments)
- [x] Edge cases handled (regex anchoring verified — ^internal/search$ doesn't mask bleveindex/searchparser)
- [x] Error handling — n/a

## Test Quality

- [x] New config tests: parallel, table-driven, external test package, no sleeps (event delivery left to the watcher's own tests)
- [x] No hardcoded values / only what matters / object-derived comparisons — followed

## Manual Verification

- [x] `go-test-coverage --config` PASS on regenerated profile (total 75.4%)
- [x] Negative check: scratch package with untested function fails the 50 floor with a clear violation line (probe removed)
- [x] Reviewer independently re-measured all override numbers against the profile — exact matches

## Quality

- [x] Follows the file's existing conventions (anchored regexes, ~5pp headroom, visible-not-silent gaps)
- [x] No security issues; the new config tests pin the validateName traversal guards
- [x] No silent failures; no debug code
