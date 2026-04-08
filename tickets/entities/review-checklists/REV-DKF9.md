---
id: REV-DKF9
type: review-checklist
title: 'Review: Add OpenAI-compatible AI client config and Lua bindings'
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

**Review Responses:**

12 design-review RRs from the planning phase (all `addressed`): RR-JV6M (AC #15
cleanup), RR-Z5XJ (temperature=0), RR-8HSQ (typed errors), RR-U833 (API key
construction timing), RR-SNJ8 (Provider interface naming), RR-LUXQ (LState
concurrency invariant), RR-6FMU (provider divergence), RR-TH21 (stream defense),
RR-GIK4 (Content-Type validation), RR-LQ1R (threat model reframing), RR-N3OU
(key leak surface), RR-QIEC (operational logging).

19 code-review RRs from the cranky-code-reviewer pass after implementation:

| RR | F# | Title | Status |
|----|----|-------|--------|
| RR-ZUPC | F1 | Provider rebuilt per validation rule per entity | addressed (un-wired AI from validation) |
| RR-T96M | F2 | 5s validation timeout silently caps AI calls | addressed (un-wired) |
| RR-IA4B | F3 | Unbounded AI spend from validations | addressed (un-wired) |
| RR-RA9I | F4 | Body-cap returns ErrNetwork not ErrBadResponse | addressed (errBodyTooLarge sentinel) |
| RR-GK2Z | F5 | Missing choices[0].message silently empty content | addressed (Message pointer + reject) |
| RR-8VL3 | F6 | base_url allows query-string credentials | addressed (Validate rejects RawQuery) |
| RR-UQ5H | F7 | No HTTP redirect policy | addressed (CheckRedirect: ErrUseLastResponse) |
| RR-U20G | F8 | LoadProvider fail-soft wrong for script/flow | addressed ((Provider, error) signature) |
| RR-9L8P | F9 | Lua error table drops cause | addressed (added details field) |
| RR-QH8C | F10 | captureLog global mutex | **deferred** (architectural; needs logger DI refactor) |
| RR-TDHV | F11 | Entry points copy-paste .rela magic string | addressed (Paths().CacheDir / project.CacheDir) |
| RR-SRCH | F12 | errBodySnippetBytes stale comment | addressed (cleaned) |
| RR-HWUO | F13 | Pointless jsonUnmarshal wrapper | addressed (deleted, inlined json.Unmarshal) |
| RR-57JZ | F14 | root.go drive-by | addressed (reverted by rebase; develop has wrapDiscoverError) |
| RR-2RJW | F15 | Body-cap test accepts two classifications | addressed (asserts exactly ErrBadResponse) |
| RR-0Z1M | F16 | Weak TestLuaAI_CompleteRejectsNonString | addressed (asserts "string expected") |
| RR-W876 | F17 | Sentinel leak test gaps | addressed (added _SuccessPath, _NetworkError) |
| RR-OD1W | F18 | parseChatRequest double-reads model | addressed (single-read style) |
| RR-GLT3 | F19 | Lua context cancellation pre-existing | addressed (resolved by PR #329 merge) |

**Summary**: 30 of 31 review-responses `addressed`. 1 `deferred` (F10/RR-QH8C —
captureLog global mutex; documented as a follow-up to inject `*slog.Logger` into
`ai.Provider`). All critical and significant findings closed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1. Config loads from .rela/ai.yaml | PASS | TestLoadConfig_Valid + TestLoadConfig_NoAPIKeyEnv. Live: smoke test against ollama. |
| 2. ai.chat returns populated result | PASS | TestLuaAI_ChatSuccess. Live: gemma3:12b returned "Four." for "What is 2+2?" |
| 3. ai.complete returns string | PASS | TestLuaAI_CompleteSuccess. Live: returned "hello from gemma" |
| 4. Missing config returns not_configured err | PASS | TestLuaAI_NotConfiguredError + TestLuaAI_NotConfiguredError_Complete |
| 5. Malformed config: script/flow fail at startup | PASS | Verified end-to-end: writing 'not: valid: yaml' to .rela/ai.yaml and running `rela script` printed "ai: parse /path/to/ai.yaml: yaml: mapping values are not allowed in this context" |
| 6. Network/HTTP errors return typed err | PASS | TestProvider_Chat_RateLimited, _AuthFailed, _BadRequest, _ServerError, _NetworkError, _Timeout, _StreamingResponse, _HTMLResponse, _MalformedJSON, _EmptyChoices, _ChoiceWithoutMessage |
| 7. temperature=0 distinct from unset | PASS | TestProvider_Chat_TemperatureZeroSentDistinctly + TestLuaAI_TemperatureZeroPropagated (request body capture) |
| 8. Lua sandbox does not regress | PASS | go test ./internal/lua/... clean |
| 9. Coverage ratchet | PASS | internal/ai 93.8%; coverage-check passes |
| 10. Smoke-tested end-to-end | PASS | Real ollama gemma3:12b round-trip + apfel error-path divergence checks |

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: documentation is in CLAUDE.md and the ai-integration concept entity, not in user-facing guides which don't exist for the Lua API yet — TKT-CVG6 covers that)
- [x] User-facing documentation updated (CLAUDE.md AI Integration section)
- [x] ~~Docs-checklist marked as done~~ (N/A: no separate docs-checklist created)

**Docs Checklist:** N/A — see above. CLAUDE.md and the `ai-integration` concept
entity carry the user-facing reference; the Lua API reference docs are tracked
separately under TKT-CVG6.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/336

All 12 CI checks green: Architecture, Build, Coverage Baseline Guard, Docs,
Frontend, Frontend Coverage Baseline Guard, Fuzz, Lint, Lint Markdown, Rela
Tickets, Test (+ auto-merge skipping).
