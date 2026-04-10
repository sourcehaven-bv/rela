---
id: PLAN-YHPYB
type: planning-checklist
title: 'Planning: Add embeddings support: ai.embed Lua binding and Provider.Embed'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Add Embed to Provider interface, OpenAI-compat HTTP implementation,
ai.embed Lua binding, config embedding_model field. Out of scope: caching,
vector store, CLI commands, validation rules.

**Acceptance Criteria:** 12 criteria in TKT-5FYM covering interface, HTTP,
config, Lua binding, security, coverage.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:** No Go library needed — direct HTTP to OpenAI-compat
/embeddings endpoint, same pattern as Chat. Reuses existing openAICompatProvider
infrastructure.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Follow Chat pattern exactly. Refactor shared HTTP
infrastructure (executeRequest, buildJSONRequest, validateResponse). New
openai_embed.go for embed-specific wire types and method.

**Files to modify:** provider.go, config.go, openai.go (refactor),
openai_embed.go (new), lua/ai.go, lua/ai_test.go, openai_embed_test.go (new),
CLAUDE.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Lua scripts provide input texts. Validated:
empty input, empty strings, batch cap (2048), model string. Same redactKey
defense-in-depth as Chat.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Edge Cases:** Empty input, empty strings, batch limit, out-of-order response
indices, count mismatch, empty vectors, missing usage, malformed JSON.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** float32 precision loss (mitigated: use float64), batch resource
exhaustion (mitigated: MaxEmbedInputs cap), polymorphic return confusion
(mitigated: always array-of-arrays).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: CLAUDE.md updated directly)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 10 findings from cranky-code-reviewer. 2 critical
(float64, batch cap), 4 significant (polymorphic return, empty input, config
interaction, index ordering), 4 minor. All addressed in implementation.
