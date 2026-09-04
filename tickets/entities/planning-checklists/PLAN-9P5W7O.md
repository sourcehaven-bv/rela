---
id: PLAN-9P5W7O
type: planning-checklist
title: 'Planning: Clear the 4 open go/path-injection and 1 js/xss-through-dom CodeQL alerts'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

> Filled in retrospectively: this was a scoped lint-clearing task worked from a
> written brief, so the planning below records the decisions actually taken
> rather than predicting them.

**Scope:**

IN: the 5 open CodeQL alerts — 4 `go/path-injection` (`internal/storage`) and 1
`js/xss-through-dom` (`frontend/src/utils/markdown.ts`).

OUT (deliberate, per the brief):
- No change to `.github/workflows/codeql.yml`. Narrowing the analyser to
  silence findings is the failure mode this work exists to avoid.
- No change to `internal/storage`'s symlink posture. `RootedFS` documents
  symlink resolution as out of scope ("the threat model is caller-supplied key
  contains traversal syntax, not attacker has write access to the root"). If
  that is wrong it is its own ticket, not a drive-by change here.
- No streaming method on the `FS` interface. `OpenForWrite`'s direct
  `os.OpenFile` is deliberate and guarded by `SupportsStreaming()`; routing it
  through `r.fs` is a larger design change.

**Acceptance Criteria:**

1. Alerts 29/31/32/33 no longer reachable — the taint path from a
   caller-supplied key to an `os.*` call runs through a barrier the analyser
   recognises. Test: `TestRootedFS_ReachesFSOnlyThroughValidatedFS` fails if
   any raw-FS or direct-`os` call reappears in `rooted.go`.
2. Alert 20 fixed by owning the escaping rather than relying on mermaid's.
   Test: hostile-SVG cases assert no surviving handler/script/`javascript:`.
3. **Diagrams still render with labels.** Test:
   `preserves foreignObject label text and SVG shape markup`, plus a real
   browser check against real mermaid.
4. Nothing dismissed rather than fixed.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: scoped lint-clearing task
worked from a written brief that already surveyed the options.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: both
fixes are local to this codebase's own types.)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small, well-specified change.

**Existing Solutions:**

- **Prior art in-tree:** FEAT-ZPGGK already describes exactly this ("a single
  `resolve(key)` method is the path-validation barrier, visible to CodeQL"),
  and TKT-TX53E anticipated CodeQL query-set expansion over the same code. This
  ticket implements the barrier half of that feature.
- **Guard-test pattern:** `internal/acl/ceilingguard_test.go` scans its own
  package for a bypass shape and fails on it, using an exemption list so new
  files fail closed. `TestRootedFS_ReachesFSOnlyThroughValidatedFS` follows it.
- **DOMPurify** is already a dependency (used by `renderMarkdown`), so no new
  library was needed for the frontend half.
- **No suppression precedent:** `grep -rn 'codeql' --include='*.go'` returns
  nothing, so annotating the alerts away would have set a new precedent. The
  brief also rates suppression below the type change.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

*Go.* `resolve()` returns a `ValidatedPath` (unexported field, no exported
constructor) rather than a `string`, and `RootedFS` reaches the wrapped `FS`
only through a `validatedFS` adapter whose methods take one. A single
`contain()` chokepoint mints every `ValidatedPath`, cleaning the joined path
and verifying it is still under the root — turning "the segment rules are
exhaustive" into a checked postcondition rather than a claim.

*Frontend.* Sanitize mermaid's SVG through DOMPurify before insertion, with
`ADD_TAGS: ['foreignObject']` and the SVG profile only.

**Alternatives rejected:**

- *Suppression comments* (`// codeql[go/path-injection]`): the brief's own
  fallback. Rejected — buys silence, not safety, and there is no precedent.
- *Making `OsFS` validate*: wrong layer. `OsFS` is deliberately an unguarded
  `os.*` wrapper; containment belongs in `RootedFS`.
- *A pre-parse with `DOMParser.parseFromString(svg, 'image/svg+xml')`*: tried
  first, and it is what discards labels — the XHTML children of
  `foreignObject` are invalid in the SVG namespace. Do not reintroduce.
- *Dropping to `textContent`*: the SVG must be parsed as markup to render.

**Files to modify:**

- `internal/storage/validated.go` (new), `rooted.go`, `validated_test.go`
  (new), `rooted_test.go`
- `internal/project/context.go`, `context_test.go`,
  `internal/templating/fstemplater.go`
- `frontend/src/utils/markdown.ts`, `markdown.test.ts`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **HTTP URL path** (`handleV1DynamicRoutes`, `api_v1.go:139`) — the single
  taint source CodeQL reports for all four Go alerts. Becomes an entity ID,
  then an fsstore key. Validated by `entity.ValidateID` and again by
  `RootedFS.resolve`; invalid input is rejected with an error, never joined.
- **Entity template variant** — reaches `EntityTemplateVariantPath` from
  `entitymanager.CreateOptions.Variant`, and automation interpolates
  `{{new.kind}}` (an API-settable entity property) into it. Allowlist
  (alphanumeric/hyphen/underscore); invalid input returns an error.
- **Diagram source** — user-authored markdown body content, rendered by
  mermaid then sanitized.

Allowlist throughout, deliberately: enumerating the dangerous forms
(separators, `..`, NUL, drive letters, Windows reserved names) is a list you can
be wrong about, and being wrong fails open.

**Security-Sensitive Operations:**

- File read/write/delete under a project root (`RootedFS` → `os.*`). Protected
  by `resolve()`'s segment rules plus the `contain()` postcondition.
- `os.OpenFile` in `OpenForWrite` — deliberate, guarded by
  `SupportsStreaming()`, now takes a `ValidatedPath`.
- HTML insertion of third-party-rendered SVG into the DOM.

Errors name the offending key or path but no file contents; a resolve failure
is an argument error, not an oracle over the filesystem.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 → `TestRootedFS_ReachesFSOnlyThroughValidatedFS` (source scan),
  `TestValidatedFS_UnwrapsToUnderlyingFS`, existing `storetest` conformance.
- AC2 → the hostile-SVG cases (handler, script, `javascript:`, media in a
  label).
- AC3 → `preserves foreignObject label text and SVG shape markup`,
  `preserves SVG <text> labels used by sequence diagrams`,
  `keeps each label with its own shape`, plus a real-browser check against
  real mermaid rendering flowchart / sequence / `stateDiagram-v2`.
- AC4 → `gh api .../code-scanning/alerts?state=open` shows the alerts as
  `fixed`, not `dismissed`.

**Edge Cases:**

- **Root is `/`** — `root + separator` is `//`, which no path matches, so every
  key reads as an escape. Found by a real fsstore test failure, not by
  inspection. Pinned by `TestContain_RootIsFilesystemRoot`.
- **Sibling-prefix path** (`/root-sibling/x` against root `/root`) — must be
  rejected; a naive `HasPrefix` without the separator boundary accepts it.
- **Empty variant** — the documented "no variant" case, must keep working;
  empty entity type is a caller bug and must be rejected.
- **Malformed SVG** — must yield an empty diagram, not a throw.
- **Sequence vs flowchart diagrams** — different label mechanisms (`<text>` vs
  `<foreignObject>`), so both must be covered; testing one hides the bug.

**Negative Tests:**

`TestContain_RejectsEscape`, `TestRootedFS_Resolve_Rejects`,
`TestContextEntityTemplateVariantPath_RejectsTraversal` (traversal, separators,
NUL, spaces, dots), and the four hostile-SVG cases. All fail with an error
return; none panic or silently pass through.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **The sanitizer silently breaks diagrams.** The main risk, and it
  materialised: the obvious fix strips every flowchart and state label while
  leaving shapes, so it looks fine. Mitigated by verifying against real mermaid
  in a real browser, covering all three diagram types, and mutation-testing
  both load-bearing config choices.
- **`ValidatedPath` ripples too far for one PR.** Mitigated by keeping
  `AbsPath`'s `string` return (its one consumer compares paths, never does
  I/O), so the change stops at the `storage` package boundary.
- **The type change looks like a fix without being one.** Mitigated by
  auditing every raw-`storage.FS` caller first, and by `contain()` being a real
  runtime postcondition rather than only a compile-time cast.
- **happy-dom vs jsdom disagree on DOMPurify**, so tests can pass against
  behaviour real browsers do not have. Mitigated: this file already opts into
  jsdom (BUG-SQSV6V).

Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] ~~User-facing docs identified~~ (N/A: internal hardening — no CLI, API,
metamodel or UI surface changes. Rationale lives in the code's doc comments,
which is where the next person editing the sanitizer will look.)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A:
`chore` kind, not an enhancement or docs ticket.)

**Documentation Impact:**
<!-- Which docs need updating? Check all that apply:
- [ ] docs/metamodel.md - New metamodel features
- [ ] docs/cli-reference.md - New/changed commands
- [ ] docs/data-entry.md - UI changes
- [ ] CLAUDE.md - New patterns or conventions
- [ ] README.md - Project-level changes
- [x] N/A - Internal change, no user-facing docs needed
-->

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the
approach was specified in a written brief that had already weighed the type
change against suppression. The one design decision taken *against* the brief —
the frontend sanitizer config — was settled by measurement rather than review.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — see above. CodeQL itself acted as the review
on the first push: it flagged the moved sink and the pre-parse step, and both
were fixed rather than suppressed.
