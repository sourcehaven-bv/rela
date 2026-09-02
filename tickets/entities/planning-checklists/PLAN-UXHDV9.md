---
id: PLAN-UXHDV9
type: planning-checklist
title: 'Planning: analyze: flag relation files whose filename disagrees with their content'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `fsstore` keys relations **entirely on the filename** —
`syncRelations` parses `FROM--TYPE--TO.md` and never opens the file. A file whose
name and content disagree is indexed under the *filename* triple, and the
relation its content describes does not exist in the graph.

Reported from a real project (GitHub issue #1004): the symptom was
`PRS-FUNC-0Q7E must have at least 1 ... relation(s), has 0` — a cardinality error
naming the **victim** entity, with no route back to the malformed file.

**Scope — IN:** a detector. `rela analyze relation-files` compares each relation
file's name against its frontmatter and names the file.

**Scope — OUT:**

- The rename fix. Already correct: `fsstore.renameEntity` writes each incident
  relation under its new name and removes the old one (verified — after
  `REQ-1 → REQ-999` the only file is `SOL-1--implements--REQ-999.md`). So the
  issue's first suggestion needs no work.
- Keying relations on content instead of filename (the issue's third suggestion)
  — a storage-model change with migration implications.
- Auto-repair. A detector that also rewrites files is one an operator cannot run
  to *find out* whether they have a problem.

**Acceptance Criteria:**

1. The reporter's exact file shape produces a finding naming the file and **both**
   triples.
2. No false positives on the repo's real projects (`tickets/`, `docs-project/`).
3. The filename split agrees **exactly** with fsstore's indexer, including
   relation types containing `--`.
4. It runs where operators look: a subcommand under `rela analyze`.

## Research

- [x] For larger features: run `/research`
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — one check modelled on two existing ones.

**Existing Solutions:**

- `analysis.CheckRelationOrder` — the sibling soft-finding check: same service,
  same shape, same CLI rendering.
- `analysis.FindOrphanedTempFiles` — the precedent for an FS-touching check,
  including the optional `FS`/`Paths` gate that returns nil rather than erroring.
- `fsstore.scanRelationDir` / `parseRelationFilename` — the code being checked.
  Reading it is what established the root cause: the index is built from filenames
  with **no file reads**, so a mismatch is not merely unreported, it is
  structurally invisible.
- `internal/frontmatter` — the dependency-free splitter `markdown` and `fsstore`
  already share. *(Found via review; the first version hand-rolled a scanner —
  see the implementation checklist.)*

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** `Service.CheckRelationFilenames` reads the relations
directory, and for each `.md` file compares the filename triple against the
frontmatter triple, returning typed findings.

**Files to modify:** `internal/analysis/relation_filename.go` (new),
`internal/cli/analyze.go`, `.go-arch-lint.yml`, plus tests and CLI docs.

**Alternatives considered:**

1. *Put the check in the store.* Rejected — the store has no reporting surface,
   and a load-time reconciliation is a behaviour change (which files load), not a
   diagnostic.
2. *A metamodel validation rule.* Rejected — validations run over the *graph*, and
   this problem is invisible in the graph by definition.
3. *Auto-repair on detection.* Rejected — see Scope.

**Dependencies:** `internal/frontmatter` added to `analysis.mayDependOn` (a leaf
`markdown` and `fsstore` already depend on); `yaml` is a `commonVendor`.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** relation files in the project the caller already
has read access to. The check is **read-only** — it opens files and reports; it
writes nothing.

**Security-Sensitive Operations:** none, with one property worth stating: an
unreadable file is skipped rather than reported. That is deliberate and not
merely convenient — fsstore treats git-crypt-encrypted relation files as locked
shells, so an unreadable relation file is *expected* in an encrypted repo, and
reporting it would produce a finding on every file in such a project.

Findings echo file paths and entity ids, both of which the caller can already
list. No file content beyond the three identity fields is read into a finding.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

- **AC1** — a dedicated test using the reporter's exact filename and content.
- **AC2** — run against the real `tickets/` and `docs-project/` projects. *This is
  the one that matters most* and it cannot be a unit test: fixture data is chosen
  by the same person holding the wrong mental model.
- **AC3** — an identical table in both packages pinning the split, including
  `A--we--ird--B` (which a naive `Split("--")` gets wrong).
- **AC4** — verified by running `rela analyze relation-files`.

**Edge Cases:** unparseable filename; relation type containing the separator;
non-markdown files; files with no relation frontmatter; CRLF; quoted values;
`---` inside the body; the legacy `type:` key.

**Negative Tests:** a consistent file must produce nothing — otherwise the check
is noise; and a real project must produce nothing, which is the strongest form of
that assertion.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

| Risk | Mitigation |
|---|---|
| False positives make the check unusable | Run against real projects before shipping — this is what caught the critical finding |
| The split drifts from the indexer, reporting good files as bad | Identical pin tables in both packages, each naming the other |
| Reading the file differently from the store creates false negatives | Use `internal/frontmatter`, the shared splitter |
| A detector nobody runs | Wired as an `analyze` subcommand and documented with the symptom operators actually arrive with |

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs-project/entities/guides/GUIDE-cli-reference.md` — new
      `rela analyze relation-files` section, regenerated into
      `docs/cli-reference.md`.
- [x] ~~CLAUDE.md~~ (N/A: no new pattern)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one check
      modelled on two existing ones in the same package. The judgement calls —
      detector not repair, analysis not store — are recorded under Alternatives.
      Worth noting the review that *did* run found a critical defect in the
      implementation, not the design: the shape was right, my reading of the
      real-project output was wrong.)
- [x] All critical/significant findings addressed in plan — 7 code-review
      findings, all addressed. See the review checklist.

**Design Review Findings:** N/A as a separate pass; code-review findings are
RR-YB7XX8 (critical), RR-DDZ02R, RR-TSBK9Z, RR-OW2QSH (significant), RR-8JRYEE,
RR-DHOSFM (minor).
