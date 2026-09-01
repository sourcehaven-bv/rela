---
id: PLAN-WFLYKO
type: planning-checklist
title: 'Planning: Extract fileLayout (and mdCodec) off fsstore.FSStore'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: `internal/store/fsstore` — step 1 `fileLayout` (6 immutable
path/key/ schema-resolution methods), step 2 `mdCodec` (8 markdown read/write
methods, same PR, only if step 1 fully green); directive 95 → 89 → 81. OUT:
tx.go, emit/observer notification, watcher/reconciler, index maps, attachment
cluster, any lock or event-ordering change, any `store.Store` surface change.

**Acceptance Criteria:**
1. `just plimsoll` with the lowered directive.
2. `go test ./internal/store/...` green — the storetest conformance harness
plus fsstore's gitcrypt/recovery/persistence/fuzz suites ARE the behavior spec;
full `go test ./...` and `-race` on fsstore too.
3. arch-lint/comment-lint/golangci-lint clean.

## Research

- [x] ~~Run `/research`~~ (N/A: mechanical extraction; full struct survey done instead)
- [x] ~~Searched for existing libraries~~ (N/A: no new functionality)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — Explore survey of all 95 FSStore methods: per-file
clusters, exclusive field ownership, ranked candidates with an explicit
do-not-touch list (recorded in the arc roadmap).

**Existing Solutions:** `urlHelpers`/`mdHelpers` shape for the immutable
collaborator; `storeutil` already hosts the shared cross-backend helpers
(fsstore uses it broadly — no new duplication introduced).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** fileLayout = pure function of config fixed at `New`
(entityFileKey/relationFileKey/propertyOrder/absPath +
buildPluralToTypeMap/resolveEntityType); no lock, no mutable state, no events —
verified in the survey against each method body. mdCodec = {rooted, layout}
owning the markdown file IO including the git-crypt inaccessible-shell path
(better cohesion: the codec owns unreadable-file handling). Alternatives
rejected for this PR: attachmentStore (−8 but called under s.mu from entity.go —
needs the lock-contract decision), indexStore (−14 but the maps are shared with
watcher.go — two-phase later), eventBus (explicitly rejected in the survey: 1
method for a second lock on every write path and changed emission ordering).

**Files to modify:** internal/store/fsstore/{filelayout.go (new), fsstore.go,
index.go, markdown.go (+ mdcodec.go or in place)}

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Unchanged — path containment stays in
`storage.RootedFS`, which moves by reference, not by reimplementation.

**Security-Sensitive Operations:** git-crypt handling (unreadable encrypted
files become inaccessible shells, never plaintext leaks) moves verbatim with the
codec; pinned by gitcrypt_test.go + gitcrypt_integration_test.go.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: storetest conformance harness is the integration spec)

**Test Scenarios:** storetest.RunAll + fuzz functions; fsstore's
recovery/persistence tests pin index-cache freshness; gitcrypt tests pin the
inaccessible-shell path.

**Edge Cases:** None new — moved methods are pure config functions (step 1) and
lock-free file IO (step 2); the risky clusters are explicitly out of scope.

**Negative Tests:** Existing conflict-marker/git-crypt/unreadable-file tests
pass unchanged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low for step 1 (immutable, no lock). Low-medium for step 2 —
mitigated by the survey's verification that the codec methods touch no mutable
index state, by the explicit skip instruction if that turns out wrong, and by
the storetest harness. Effort: m.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor kind)

**Documentation Impact:** N/A — internal change.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: plan derived from a dedicated structural survey with per-candidate risk ranking and an explicit do-not-touch list; repeats the established extraction design)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no review run)

**Design Review Findings:** N/A
