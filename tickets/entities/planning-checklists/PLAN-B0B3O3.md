---
id: PLAN-B0B3O3
type: planning-checklist
title: 'Planning: Unify the two entity-ID validators into one enforced rule'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: collapse `entity.ValidateID` + `storeutil.ValidateID` into one rule
enforced at the store boundary; reject a leading dash.

OUT: removing `{id}` from the `sh -c` string (TKT-QGHNVA); relaxing to
non-ASCII IDs.

**Acceptance Criteria:**

1. Exactly one ValidateID implementation; the other delegates.
   Test: `storeutil.ValidateID` has no independent rule (code) +
   `TestValidateID` in both packages.
2. Every write path gated by that rule. Test: `storetest` conformance +
   `FuzzRelationKeyCollision` / `FuzzRenameKeyCollapse` (bidirectional oracle).
3. Leading `-` rejected. Test: `TestValidateID/invalid/leading_dash`.
4. Storetest conformance passes for every backend.
5. No existing project breaks. Test: scan of all real entities.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small refactor)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, well-understood refactor.

**Existing Solutions:**

- Prior art in-repo: `storeutil.ValidateRelationType` / `ValidateProperty`
  already follow the "one shared rule, backends delegate" shape this restores.
- `FEAT-CO4YP` already declares ID validation a `storeutil` invariant, so
  this is restoring the documented design, not inventing one.
- External convention: DNS labels (RFC 1123), Kubernetes resource names, and
  package registries all keep identifiers ASCII while allowing Unicode in
  display names — the same split rela has via the `title` property.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Keep `entity.ValidateID` as the single owner of the grammar; make
`storeutil.ValidateID` delegate and add only the `store: ` error prefix.
Import direction permits this (`storeutil` already imports `entity`;
`entity` imports no store package), so no cycle and no new package.

Kept as a named function rather than an alias because it is the storetest
fuzz oracle and because the prefix names the layer that refused the write.

Reason-specific checks (path separator, control char) run before the
character-class check so error messages keep naming the cause — the
storetest conformance suite asserts on those phrases.

**Files to modify:**

- `internal/entity/id.go` — the single rule + the documented why
- `internal/store/storeutil/storeutil.go` — delegate
- `internal/entity/id_test.go`, `internal/store/storeutil/storeutil_test.go`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Entity IDs arrive from: HTTP request paths (data-entry), the CLI, the
importer, and Lua scripts. All now converge on one **allowlist**
(`^[A-Za-z0-9_-]+$` + no leading dash, no `--`, no `..`), which is the
preferred direction over the previous partial blocklist. Invalid input is
rejected at the store boundary with a reason-naming error.

**Security-Sensitive Operations:**

1. ID becomes a filename verbatim (`fsstore.go:372`) — path separators,
   traversal, and control chars rejected.
2. ID becomes part of the relation key `FROM--TYPE--TO.md` — `--` rejected.
3. ID can reach `sh -c` via `document.go:renderCommand` — leading dash
   rejected (argument injection). Defense in depth only; TKT-QGHNVA removes
   the exposure structurally.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

AC1 → `TestValidateID` (entity) + `TestValidateID` (storeutil).
AC2 → `storetest` conformance across fsstore/memstore + the fuzz oracle.
AC3 → `TestValidateID/invalid/leading_dash`, `/leading_dash_word`.
AC4 → `TestConformance` in each backend package.
AC5 → scan of the real `tickets/` and `docs-project/` projects.

**Edge Cases:**

- Empty string → "empty ID".
- Single character (`a`) → valid (lower boundary).
- NUL / tab / newline / DEL → "control character".
- Multi-byte UTF-8 (`café`, `日本語`) → rejected; continuation bytes are
  0x80+ so they reach the character-class check, not the control-char scan.
- Homoglyph (Cyrillic `аdmin`) → rejected.
- Zero-width space, bidi override → rejected.
- `--` (relation separator) and `..` (traversal) → rejected with own reason.

**Negative Tests:**

24 rejection cases in `TestValidateID`, each asserting the message names the
reason (not just that an error occurred), so a future refactor that widens
the class fails loudly.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. *Existing projects hold a now-invalid ID.* Mitigated by measurement:
   scanned all 2030 entities in `tickets/` + `docs-project/` — zero
   rejected. Repair path if a downstream project differs is the existing
   `Manager.RenameEntity` / `rela rename` (atomic relation re-key); there is
   no content-migration machinery (`internal/migration` is schema-only).
2. *Tightening breaks the fuzz oracle.* The oracle is bidirectional
   (`ValidateID(id) != nil` ⇒ store must reject), so backends had to
   actually reject the newly-invalid IDs. Verified by running the fuzz
   targets, not by reasoning.
3. *Reversing a deliberate Unicode decision.* Checked: the test asserting
   `café` was documenting a byte-scanning property, not a product
   requirement. Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user-facing docs)

**Documentation Impact:**
N/A — internal refactor. The rule's rationale is documented in the
`ValidateID` godoc (where the next reader will be) rather than in prose docs.
No user-facing behaviour changes for any conforming ID.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: approach agreed with user directly)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Approach agreed with the user before
implementation (ASCII-only vs Unicode-tolerant was explicitly discussed and
decided). No separate `/design-review` run for a change of this size.
