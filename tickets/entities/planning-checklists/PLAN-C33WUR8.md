---
id: PLAN-C33WUR8
type: planning-checklist
title: 'Planning: LLM actions: follow-ups (auto-apply, create outputs, batch runs)'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `apply: auto` (background job applies the proposal as the user),
`outputs.create`, list `actions:` and flow job steps for auto actions,
server-wide `-ai-config`, candidate condition pushdown, lowering `max` by
existing relations (RR-ZXPYDQJ). Out: relation properties in proposals, faced
subject or create types, a review queue for auto-applied changes.

**Acceptance Criteria:**
1. An `apply: auto` run from the SPA queues a job and answers at once; the job applies the proposal as the queuing user (TestLLMAction_AutoQueues, TestLLMActionRunner_Applies).
2. A refused or misconfigured background run writes nothing and is not retried (TestLLMActionRunner_Refusals, TestHandleLLMActionJob).
3. `outputs.create` proposes new entities; the dialog and the background run create them and relate them (TestLLMActionRunner_Creates, LLMProposalDialog create test).
4. Lists and flows accept only auto actions on their type (TestValidateListActions_LLM, TestFlowActionUsable_LLM).
5. A project without ai.yaml uses the server-wide one; tenant withholding still applies (TestWithServerAIConfig).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: follow-ups to the design of TKT-4USH1HO)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: built on rela's own jobs and write seams)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- jobs.Queue with RetryBounded and IdempotencyKey, as background automation actions.
- attachRenderHolder for a late-bound runner installed by the data-entry host.
- entitymanager AuthorizeUpdate + PatchGranted, as the PATCH handler.
- hostconfig wrapping, as withheldHost for tenant mode.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** The route enqueues an `llm-action` job naming action,
entity and user. The handler stamps the principal, binds the ACL request and
calls a runner the data-entry app installs. The runner runs the same gates and
model call, then applies the proposal through entitymanager, skipping stale
properties. Retry is decided by status (409/502/503/504). Alternative rejected:
applying inline on the request, which holds the request open for minutes and
puts an external call on the write path.

**Files to modify:** internal/dataentry/actions_llm*.go,
internal/appbuild/llmactionjobs.go, flowjobs.go, serverai.go,
internal/dataentryconfig, internal/llmtask, internal/dataentrywire/wire.go,
cmd/rela-server/main.go, frontend LLMProposalDialog, docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The model reply is checked against the derived
schema and the metamodel (allowlist of properties, candidates, types). The job
payload names an action and grants nothing; the action comes from live config.
The server ai.yaml is parsed at startup.

**Security-Sensitive Operations:** Background writes run as the queuing user
with that user's ACL, validation and audit. Reads are gated by attaching the
bound read gate (fails closed without identity). Auto-apply has no human review,
so prompt injection can steer free-text values; documented, and bounded outputs
are recommended.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria.

**Edge Cases:** Property changed while the model answered (skipped); no queue
(503); no user (403); full relation (no room in schema); all-null created entity
(dropped).

**Negative Tests:** Unpermitted user, unknown action, proposing action in
background, unbound context, invalid or missing server ai.yaml, list or flow
with a proposing action.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Prompt injection on auto-applied free text (documented; permission
required). Non-atomic multi-write apply (same as the dialog; each write
protected by the manager).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-data-entry (AI actions), GUIDE-flows (job step
action).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the follow-ups extend the design reviewed under TKT-4USH1HO; the code review covers the new parts)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
