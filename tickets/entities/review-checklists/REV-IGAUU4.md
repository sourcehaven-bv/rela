---
id: REV-IGAUU4
type: review-checklist
title: 'Review: Multi-enum list filter renders a native listbox and never matches any row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go test ./...` — all pass
- [x] `just lint` — 0 issues
- [x] `just arch-lint` — OK, no warnings
- [x] `just comment-lint` — no unresolvable doc links (11080 comments)
- [x] `just coverage-check` — 78.3%, package and total thresholds pass
- [x] `just plimsoll` — clean
- [x] frontend `vitest` — 2011/2011 pass
- [x] `vue-tsc --noEmit` — clean
- [x] `eslint` — 0 errors
- [x] `prettier --check` — clean

`arch-lint` caught a real boundary violation mid-review: an interim version
imported `internal/propmatch` from `internal/dataentry`, which the ruleset
forbids. Reverted to a local `%v` render with a comment recording that the two
must agree, and that TKT-UTJ24Z is where they converge for real.

## Code Review

Performed with the cranky-code-reviewer agent. **Eight findings: 2 critical, 3
significant, 2 minor, 1 nit.** All eight are recorded as linked
`review-response` entities. Every claim was independently reproduced before
being accepted — two of the three "significant" ones turned out to be
pre-existing rather than introduced here, which changed how they were handled.

| Finding | Severity | Status |
| --- | --- | --- |
| RR-CUW2W5 — `Nil:` contract said the opposite of the code | critical | addressed |
| RR-4KY35T — comma-bearing value matched wrong rows; `ne` failed to exclude | critical | addressed |
| RR-HGKI3S — `propertyElements` duplicated `propertyContains` | significant | addressed |
| RR-A5MXIP — ordered operators answered with garbage instead of declining | significant | addressed |
| RR-NTSTPD — empty `in`/`ne` is not the complement of a populated one | significant | deferred → TKT-UTJ24Z |
| RR-D3DH50 — `in`/`ne` trim the filter side but not the property side | minor | deferred → TKT-UTJ24Z |
| RR-3I93HB — test table missed the edge cases | minor | addressed |
| RR-5EPBE8 — quote-style churn on a CSS line | nit | wont-fix (Prettier-mandated) |

**The comma finding was the important one.** The first cut joined repeated
params on commas and split them back, so `"Legal, Risk & Compliance"` was torn
in two: `in` returned the wrong entity and `ne` returned the row it was asked to
exclude — the exact wrong-answer class this bug exists to fix, reintroduced
through a different door, and made reachable by routing multi-select through
`in`. Fixed on both sides: the backend keys off the explicit `[]` array suffix
(not the param count — a one-element array is indistinguishable from a comma
list by count, which is why the first fix attempt still tore the value), and the
SPA keeps a single selection on `=`, which compares whole and is always
comma-safe.

**Two deferrals were argued, not waved through.** RR-NTSTPD and RR-D3DH50 are
both real, and both were verified byte-identical before and after this change —
they live in the top-of-loop missing-property branch and an inherited trim, not
in the element-wise comparison. Fixing them alters SCALAR filtering on a path
this bug does not otherwise touch, which is precisely the blast-radius argument
that split TKT-UTJ24Z out in the first place. Both are now written into that
ticket's scope with reproductions.

## Acceptance Verification

Re-verified end-to-end after the review changes, against a running server and
the real SPA.

| Criterion | Evidence | Result |
| --- | --- | --- |
| Multi-enum filter is a tag/chip control, not a native listbox | Single-line dropdown with search; selections render as removable chips with ✕ | PASS |
| Selecting a value filters correctly | `Informatiebeveiliging` → 2 of 3 rows, the ones that carry it (was: "No … found") | PASS |
| Multi-selection works | + `Strategie` → all 3 rows, URL `filter[gebieden][in]=Informatiebeveiliging,Strategie` | PASS |
| Single selection is comma-safe | URL stays `filter[gebieden]=Informatiebeveiliging` (`=` compares whole) | PASS |
| Deep links work | Cold load of `?filter[gebieden][in]=Strategie` hydrates the chip, returns 1 row | PASS |
| `ne` excludes correctly | `ne=Informatiebeveiliging` → only the row without it (was: all 3) | PASS |
| Scalar filtering unchanged | `filter[status]=concept` → 1 correct row; dedicated regression test | PASS |
| Null property is not `"<nil>"` | `contains=nil` → no rows; `filter[tags]=` matches the null row | PASS |
| Comma-bearing value | array form matches the right row under `in`, excluded under `ne` | PASS |
| Ordered op on a list | matches no list-valued row, logs a warning | PASS |

## Verification

- [x] All critical and significant findings addressed or argued-and-deferred
- [x] No open critical/significant responses
- [x] Regression tests proven to fail without the fix (7 of 8 original cases)
- [x] Manual end-to-end verification repeated after review changes
- [x] Deferred findings recorded in TKT-UTJ24Z with reproductions
