---
id: REV-WQ37P
type: review-checklist
title: 'Review: Document the documents feature and add Lua script renderer'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

**Evidence:** `just check` passes; `just lint` = 0 issues; `just coverage-check`
= 73.7% total (threshold 65%); `just arch-lint` clean. E2E tests run separately
(tag-gated); `TestE2E_LuaDocumentRenders` passes in 6.77s.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:**

33 review-responses total (16 from pre-impl design-review + 17 from post-impl
cranky + go-architect reviews):

- **Critical (2) — addressed**: RR-4QSBN (singleflight key), RR-FLCXC (EntityType enforcement).
- **Significant (9) — addressed**: RR-1FA8W, RR-I5WME, RR-UPOQZ, RR-FTFJU, RR-J3KA9, RR-25XXM, RR-DWZKU, RR-82D0N, RR-DBCF1, RR-BOV25.
- **Minor (5) — 1 addressed (RR-1FG6X), 4 deferred** to backlog tickets (TKT-IMBOK hot-reload checks, TKT-5LCNM script-path plumbing, TKT-GOLNP stub EntityManager extraction, TKT-96VGO cache footgun).
- **Nit (13) — 9 addressed, 3 won't-fix** (RR-36PGJ, RR-LPEMA, RR-VLCKA — all with documented reasons), 1 paired (RR-HSEJ6 addressed alongside RR-BOV25).

No unrelated changes in the diff; pre-existing E2E helper signature drift
(unrelated `NewApp` change on develop) fixed as a drive-by because the new test
needed a working helper.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 Lua happy path — PASS (`TestDocumentService_ScriptRender_CapturesMarkdown`)
- AC2 Config validation — PASS (`TestValidateConfig_Documents` 6 subtests)
- AC3 Context injection — PASS (`TestDocumentMode_ContextInjection`, now using `rela.document.entry_id`)
- AC4 Context absent elsewhere — PASS (`TestDocumentMode_AbsentInOtherContexts`, using `rela.output` for readback)
- AC5 `rela.output` warning — PASS (`TestDocumentMode_OutputIsWarning`)
- AC6 Cache memoize across renders — PASS (`TestDocumentService_CacheMemoizeAcrossRenders`, real `script.Engine`)
- AC7 Shell command unchanged — PASS (existing tests + `TestHandleV1Documents_EntityTypeMatch`)
- AC8 Singleflight keyed on configID — PASS (`TestDocumentService_SingleflightNoCollapseAcrossConfigs` + positive complement)
- AC9 Handler enforces EntityType — PASS (`TestHandleV1Documents_EntityTypeMismatch`, `TestHandleV1Documents_EntityNotFound`)
- AC10 Disk cache bypass — PASS (`TestDocumentService_ScriptRender_NoDiskCacheWrite`, `TestDocumentService_ScriptRender_StaleCommandCacheIgnored`)
- AC11 cfg.Timeout honored — PASS (`TestExecuteDocument_TimeoutEnforced`)
- E2E — PASS (`TestE2E_LuaDocumentRenders` — full browser → HTTP → Lua → goldmark → DOMPurify)
- AC-DOC1 Guide section — PASS (manual review; `GUIDE-data-entry.md` has the Documents section with all required subsections)
- AC-DOC2 FEAT-023 updated — PASS (status: implemented, content reflects V2 Lua renderer)
- AC-DOC3 Prototype example — PASS (`prototypes/data-entry/project/scripts/docs/category_report.lua` wired into `data-entry.yaml`)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-VXCAJ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

No TODOs in the diff. Commit message will cover: *why* (data-entry documents
feature was undocumented + shell-out was high-friction for multi-entity
composition) and *what* (Lua renderer alongside shell, docs, prototype example,
tightened cache/singleflight/type-enforcement plumbing the design review
surfaced).

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: user-gated next step after review sign-off)
- [x] ~~All CI checks pass~~ (N/A: verified locally; PR-side CI runs after the branch is pushed)
- [x] ~~PR URL documented below~~ (N/A: no PR yet)

**PR:** *pending — will be created via `/pr` after user sign-off*
