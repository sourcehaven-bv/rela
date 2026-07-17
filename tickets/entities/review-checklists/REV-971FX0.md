---
id: REV-971FX0
type: review-checklist
title: 'Review: CLI table output ignores display_property (uses literal title only)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green)
- [x] Lint clean (`golangci-lint` 0 issues; `go vet` clean; `just arch-lint` OK; `just plimsoll` OK)
- [x] Coverage maintained (`just coverage-check` PASS — 77.1%)

## Code Review

- [x] Ran cranky-code-reviewer on commit 24772442
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-QM9CDV, RR-X3YE8K, RR-18E3PX)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**
- RR-QM9CDV (significant → addressed) — misleading trace comments corrected; TKT-COZN2E filed.
- RR-X3YE8K (significant → addressed) — export reverted to raw title (data-interchange); AC4 narrowed to graph.
- RR-18E3PX (significant → addressed) — graph ID-fallback guard now tested.
- RR-US77XX (minor → addressed) — guard operand, JSON asymmetry, fixture narrowing, Meta() cost, nil-path documented/justified.

## Acceptance Verification

- [x] Each acceptance criterion tested

**Acceptance Status:** (manual + unit)
1. bare-name `display_property: achternaam` → "Vloothuis" (was blank) — **PASS**
2. template → "Jeroen Vloothuis" (was blank) — **PASS**
3. literal-title tickets unchanged — **PASS**
4. graph node labels honor display_property — **PASS** (`TestGenerateDOT`, manual DOT)

Excluded by design (review): export stays raw data; trace deferred to
TKT-COZN2E.

## Documentation

- [x] ~~User-facing docs~~ (N/A: no new metamodel/CLI surface; behavior now matches the data-entry app and the already-documented display_property)
- [x] ~~Docs-checklist~~ (N/A: internal wiring)

## Final Checks

- [x] Commit messages explain the why
- [x] No TODOs/FIXMEs (trace gap tracked as a ticket, not a code TODO)
- [x] Ready for another developer to use

## Pull Request

- [x] PR created: https://github.com/sourcehaven-bv/rela/pull/1085
- [x] All CI checks pass (verified locally: build, full test, lint, vet, arch-lint, plimsoll, coverage; PR CI running)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1085
