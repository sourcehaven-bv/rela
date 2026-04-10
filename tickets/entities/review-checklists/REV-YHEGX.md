---
id: REV-YHEGX
type: review-checklist
title: 'Review: Add embeddings support: ai.embed Lua binding and Provider.Embed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Design review performed pre-implementation with 10
findings (2 critical, 4 significant, 4 minor). All critical/significant
addressed: float64 end-to-end, batch cap 2048, consistent array-of-arrays
return, empty input validation, index sorting, config fallback chain.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: Provider interface has Chat and Embed
2. PASS: Embed POSTs to /embeddings (TestProvider_Embed_PostsToCorrectEndpoint)
3. PASS: Same hardening as Chat (shared validateResponse, executeRequest)
4. PASS: Typed errors (TestProvider_Embed_HTTPError, TestProvider_Embed_MalformedJSON)
5. PASS: EmbeddingModel optional (TestProvider_Embed_FallsBackToModel, _UsesEmbeddingModel)
6. PASS: ai.embed(string) returns array-of-arrays (TestLuaAI_EmbedSingleString)
7. PASS: ai.embed(table) returns array-of-arrays (TestLuaAI_EmbedBatch)
8. PASS: Model override (TestLuaAI_EmbedModelOverride)
9. PASS: Sentinel leak test (TestProvider_Embed_KeyNeverLeaks, _SuccessPath)
10. PENDING: Live smoke test against ollama (post-merge manual)
11. PASS: Coverage 94.9%
12. PASS: Zero wiring changes needed

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: CLAUDE.md updated inline)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: no separate docs checklist)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/370
