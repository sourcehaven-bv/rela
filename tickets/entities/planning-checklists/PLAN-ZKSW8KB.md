---
id: PLAN-ZKSW8KB
type: planning-checklist
title: 'Planning: Declarative LLM actions'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: an inline `llm:` block on data-entry actions, entity-bound through
`available_on`; inputs (subject properties, content with a size cap, related
entities per relation); outputs `set` and `relate` to candidates;
schema-constrained reply; proposal returned on the action response; detail-page
proposal dialog that applies checked changes through the existing PATCH and
relation-create APIs; MCP `run_action` returns the proposal; docs.

Out (follow-ups): auto-apply on `jobs.Queue`; `create` outputs; flow step
outputs; list-toolbar and batch runs; scheduled runs; tool calling; static
`params` in the prompt; a server-level provider config separate from ai.yaml.

**Acceptance Criteria:**

1. An `llm` action on a detail page returns a proposal; nothing is written.
Test: handler test with a fake provider; store unchanged after the call.
2. A `visible:`-hidden input field never reaches the provider. Test: fake
provider records the request; the hidden value is absent.
3. A reply with an unknown property, invalid enum value or id outside the
candidates is rejected. Test: llmtask parse table tests + handler test.
4. Accepting writes only the checked changes, as the user. Test: frontend
unit test for the dialog (PATCH body has only checked properties; relation calls
only for checked rows).
5. Invalid config fails to load with a message naming the key. Test:
dataentryconfig validate table tests.
6. MCP `run_action` returns the proposal. Test: `TestActionResultView_Proposal`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: approach discussed and agreed with the user in session; options recorded in FEAT-QLU1C1B)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: OpenAI structured outputs is the reference; no library needed)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- No library: the provider is OpenAI-compatible; `response_format:
json_schema` is part of that wire format (Ollama supports it too).
- `internal/openapi/schemas.go:139` maps metamodel properties to JSON
schema; reused as the model for the schema derivation (with strict-mode changes:
every field required, nullable instead of optional).
- `actionOps.run` (`internal/dataentry/actions_ops.go:43`) gives permission,
available_on and when gates; the llm branch reuses them.
- `visibility.ScriptReader` (`internal/visibility/luareader.go`) gives gated,
redacted reads for subject, relations and candidates.
- `lua.loadAIProvider` (`internal/lua/context.go:110`) plus `Seams.AI` is the
sanctioned provider source.
- Comment suggestions (`SuggestionDiff.vue`) are the nearest
propose/accept UI.
- FEAT-ER3Y principle: AI output is a proposal, never a silent edit.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** see the Plan section of TKT-4USH1HO.

Alternatives rejected:
- Templated prompt: becomes a template language (loops, conditions),
mixes untrusted data into the instruction channel, and defeats prompt caching.
Structured JSON input instead.
- Apply directly via `jobs.Queue` in v1: needs a job status channel back to
the user and makes the AI output a silent edit. Kept as follow-up opt-in.
- A separate `llm_tasks:` registry: the first plan had one. Dropped during
implementation because each task had exactly one action; the inline block keeps
the config in one place. A registry can be added when tasks are shared.

**Files to modify:**

- `internal/ai/provider.go`, `internal/ai/openai.go` (+tests): ResponseFormat.
- `internal/llmtask/` (new): config types, compile, schema, request, parse.
- `internal/dataentryconfig/config.go`, `llmaction.go`, `validate.go`
(+tests): `Action.LLM`, checks.
- `internal/dataentry/actions_ops.go`, new `actions_llm.go` (+tests);
`internal/script/action.go` or apiwire `ActionResponse.Proposal`.
- `internal/lua/context.go`: export a provider loader for the llm path.
- `internal/mcp/tools_actions.go`: render proposal.
- `frontend/src/...`: API type, `LLMProposalDialog.vue`, EntityDetail
handling (+unit test).
- `docs-project/` entity for data-entry actions; `docs/` regenerated.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Config (operator, trusted): validated at load; unknown keys refused.
- Entity data (untrusted, may carry prompt injection): read gated as the
caller; sent only in the user message as JSON; never interpolated.
- LLM reply (untrusted): parsed as JSON, validated against an allowlist of
declared properties, enum values and candidate ids; anything else is an error.
The reply never triggers a write by itself.

**Security-Sensitive Operations:**

- Sending data to an external provider: only fields the caller may read,
only declared inputs, with size caps. Gated by action `permission`.
- Writes: only on accept, through the existing authorized APIs.
- Errors: provider errors use the redacted `ai.Error`; the reply text is not
echoed into logs.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria above. Integration: dataentry
handler test through the real HTTP route with a fake `ai.Provider` via
`Seams.AI`, plus an e2e-free manual run against a local OpenAI-compatible server
if one is available.

**Edge Cases:**

- No ai.yaml: action fails with a clear "AI not configured" error.
- Subject has no related entities / no candidates: empty arrays; relate
output offers nothing.
- Content longer than `max_chars`: truncated with a marker.
- Model returns null for a property: no change proposed.
- Model returns the current value: shown as unchanged, not proposed.
- List properties and enum lists: arrays in the schema.
- Reply not valid JSON or truncated (finish_reason length): error.
- Candidate the caller cannot read: never in the list.
- Duplicate relation already present: shown as existing, not proposed.

**Negative Tests:** invalid config combos; reply with extra property; invalid
enum; id outside candidates; more relations than `max`; provider error; context
timeout.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Providers without strict json_schema support: validate every reply in Go
and describe the schema in the system message as well.
- Large candidate sets inflate cost: bounded `limit` checked at load.
- Config surface creep: v1 keys are minimal and refuse unknown keys.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - `llm:` actions (via docs-project)
- [x] docs/lua-scripting.md / ai docs: pointer to the declarative option

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** nine findings (four significant, four minor, one
nit), RR-13MJ1TP to RR-WNQA13C. All are addressed: permission required, write
gates applied to the schema, faced types and content-scoped or property-carrying
relations refused, strict schema, stale-value check on apply, per-item failure
report, existing relations marked, timeout.
