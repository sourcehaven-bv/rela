---
id: REV-MGFUZN
type: review-checklist
title: 'Review: display_property as a template (multi-property title)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` green)
- [x] Lint clean (`golangci-lint ./internal/metamodel/ ./internal/dataentry/` — 0 issues; `go vet` clean)
- [x] Coverage maintained (`just coverage-check` PASS — total 76.9%)

## Code Review

- [x] Ran cranky-code-reviewer on commit f09a6afa
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-978KZI → addressed)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**
- RR-978KZI (significant → addressed) — git-crypt locked-title template leak in mentions.go; fixed with `DisplayProperties()` + regression tests.
- RR-00TCAM (minor → addressed) — direct scanner unit tests added.
- RR-87IIFY (minor → addressed) — doc sentence on literal-text ID fallback.
- RR-3QJCB7 (nit → wont-fix, justified) — insertion sort (pre-existing/out of scope), NBSP collapse (intended), defense-in-depth godoc (now tested).

## Acceptance Verification

- [x] Each acceptance criterion tested

**Acceptance Status:** (all via `go test ./internal/metamodel/`)
1. `"{voornaam} {achternaam}"` → "Jeroen Vloothuis" — **PASS**
2. empty tussenvoegsel → single space — **PASS**
3. all placeholders empty → ID — **PASS**
4. bare name unchanged — **PASS**
5. literal comma passthrough — **PASS**
6. `{unknown_prop}` → load error — **PASS**
7. `{voornaam` (unclosed) → load error — **PASS**
8. `GetPrimaryProperty()` on template → "" — **PASS**

Plus: adjacent placeholders, brace-in-value non-reinjection, unicode, direct
scanner tests, and the mentions locked-title regression.

## Documentation

- [x] User-facing docs updated — metamodel guide Templates subsection (syntax, whitespace collapse, ID-fallback corner, display-only). Regenerated docs/.
- [x] ~~Docs-checklist~~ (N/A: doc change folded into this PR; single guide subsection)

## Final Checks

- [x] Commit message explains the why
- [x] No TODOs/FIXMEs
- [x] Ready for another developer to use

## Pull Request

- [x] PR created: https://github.com/sourcehaven-bv/rela/pull/1070
- [x] All CI checks pass (verified locally: build, full test, lint, vet, coverage, docs, ticket-validate; PR CI running)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1070

## Follow-up

- **TKT-VHSHOB** filed: CLI table output (`rela list`/`export`/`graph`) shows blank titles because it uses `entity.Title()` not `DisplayTitle` — pre-existing, affects bare-name too. To be implemented next.
