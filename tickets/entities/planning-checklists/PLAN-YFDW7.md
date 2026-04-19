---
id: PLAN-YFDW7
type: planning-checklist
title: 'Planning: Fix misfiled entity files in docs-project/entities/'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:
- `docs-project/entities/guide/` (8 files) → merge into existing `guides/`
- `docs-project/entities/feature/` (14 files) → merge into existing `features/`
- `docs-project/entities/tutorial/` (2 files) → rename to `tutorials/`
- `docs-project/entities/scenario/` (3 files) → rename to `scenarios/`
- `docs-project/entities/concept/` (7 files) → rename to `concepts/`
- Remove the 5 now-empty singular folders

OUT of scope:
- No changes to entity file contents (YAML frontmatter, body).
- No changes to `tickets/` (already correct plural layout).
- No code changes to fsstore, no metamodel `plural:` overrides.
- No guardrail / lint rule to prevent recurrence (deferred).

**Acceptance Criteria:**

1. `docs-project/entities/` contains only plural-named subdirectories (`guides`, `features`, `tutorials`, `scenarios`, `concepts`); no singular folders remain.
   - Test: `ls docs-project/entities/` shows only the 5 plural names.
2. All 34 entity files preserved with original content — no additions, no deletions, no in-file edits.
   - Test: `git log --stat` on the commit shows only renames (`R100`), zero content bytes changed.
3. `rela` can load the docs-project workspace without type-resolution errors.
   - Test: `cd docs-project && rela list` returns every entity classified under its correct type (guide/feature/tutorial/scenario/concept) with the expected counts.
4. `rela analyze` (cardinality + orphans + properties + validations) runs without surfacing type-mismatch errors caused by the old singular folders.
   - Test: run each analyze command; none report unknown entity types like `guid` / `featur` / `tutoria` / `scenari` / `concep`.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- Pluralization convention is hard-coded in fsstore:
  - `internal/store/fsstore/fsstore.go:241-248` — `entityFilePath` joins `entitiesDir/<plural>/<id>.md`, where plural defaults to `entityType + "s"` or uses `schema.Plural` if set.
  - `internal/store/fsstore/index.go:274-291` — `buildPluralToTypeMap` + `resolveEntityType` do the reverse mapping; unknown directory names fall through to `strings.TrimSuffix(dirName, "s")`, which is why `guide/` silently becomes type `guid`.
- Neither metamodel (`docs-project/metamodel.yaml`, `tickets/metamodel.yaml`) sets a `plural:` override, so the default `type + "s"` applies everywhere.
- `tickets/entities/` already follows the plural convention correctly (spot-checked: `concepts/`, `features/`, `bugs/`, `tickets/`, etc.). Use it as the reference layout for `docs-project/`.
- No library needed — this is a one-shot `git mv` cleanup.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Use `git mv` for every file (preserves history as a rename):

```bash
cd docs-project/entities
# Merge into existing plural folders
git mv guide/*.md guides/
git mv feature/*.md features/
# Rename singular folders (no plural counterpart exists)
git mv tutorial tutorials
git mv scenario scenarios
git mv concept concepts
# Remove the empty singular folders that merged
rmdir guide feature
```

No overlapping filenames across pairs (verified with `comm -12` during
analysis), so merge is safe — no clobber, no manual resolution.

**Alternatives considered:**

- Plain `mv` instead of `git mv`: rejected. `git mv` keeps the rename visible in history so `git log --follow` still traces file provenance.
- Add `plural:` overrides in `metamodel.yaml` to accept singular folder names: rejected. That legitimizes a broken layout and diverges from how `tickets/` is set up. The convention should be one way.
- Add a lint/guardrail in fsstore to reject unknown plural folder names: out of scope (deferred). Sensible follow-up but not required to fix the data.

**Files to modify:**

- `docs-project/entities/guide/*.md` (8 files) — moved into `docs-project/entities/guides/`
- `docs-project/entities/feature/*.md` (14 files) — moved into `docs-project/entities/features/`
- `docs-project/entities/tutorial/` — renamed to `docs-project/entities/tutorials/`
- `docs-project/entities/scenario/` — renamed to `docs-project/entities/scenarios/`
- `docs-project/entities/concept/` — renamed to `docs-project/entities/concepts/`

Nothing outside `docs-project/entities/` is touched.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- No user input, no network, no new data accepted. Only moves existing tracked files within the repo.

**Security-Sensitive Operations:**

- Filesystem writes limited to `docs-project/entities/` and only to paths that already exist in `git ls-files`. Git itself enforces that only tracked files are touched; no new file contents are written.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1 (plural-only folders): after moves, `ls docs-project/entities/` lists exactly `concepts features guides scenarios tutorials` — no singular forms.
2. AC2 (content preserved): `git diff --stat develop..HEAD -- docs-project/` shows R100 (pure rename) for all 34 files; `git log -p -- docs-project/entities/` for a sample file shows no content changes introduced.
3. AC3 (rela loads correctly): `cd docs-project && rela list --type guide | wc -l`, `--type feature`, `--type tutorial`, `--type scenario`, `--type concept` each return the expected file count (8, 14 + previous 2 = 16, 2, 3, 7).
4. AC4 (analyze clean): run `rela analyze cardinality`, `rela analyze orphans`, `rela analyze properties`, `rela analyze validations` (or the MCP equivalents on rela-docs) and confirm no failures caused by the old folders. A post-move `refresh` is needed because the graph cache predates the move.

**Edge Cases:**

- ID collision between singular and plural folder — verified absent during analysis (`comm -12` returned empty for both pairs). No action required.
- Files that happen to share a name across types — N/A, files are scoped to one type folder at a time.
- Non-`.md` files inside the singular folders — double-checked: only `.md` entity files exist; no hidden files, no subdirectories.
- `.rela/cache.json` in `docs-project/` is stale after the move. Accept: rela rebuilds the cache on next sync.

**Negative Tests:**

- If any file rename fails (permission, path collision), the whole operation aborts before any `rmdir`, leaving the repo mid-state. Recovery = `git restore --staged . && git checkout -- docs-project/entities/`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Risk:** A file in a singular folder is already referenced by ID somewhere (entity body, relation file, cross-repo link) and the rename breaks that reference.
**Mitigation:** IDs come from filenames, not directory paths, so moving a file
from `guide/GUIDE-concepts.md` to `guides/GUIDE-concepts.md` keeps the ID
identical. No relation files live in these folders.
- **Risk:** Stale `.rela/cache.json` in `docs-project/` continues to show the old (mis-typed) view after the move.
**Mitigation:** Cache is gitignored. Refresh (or any sync-triggering command)
rebuilds it. Not a correctness bug.
- **Effort:** xs. Mechanical, no code, ~5 minutes of actual work plus verification.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed

This ticket is a `chore`, not an enhancement — no docs-checklist required per
the project workflow (docs-checklists are only mandated for enhancement / docs
kinds).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: xs chore, no design surface — pure data move with no code changes, no API changes, no architectural decisions)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: design review skipped)

**Design Review Findings:** none (skipped — see above)
