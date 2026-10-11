---
id: PLAN-K5DPR1
type: planning-checklist
title: 'Planning: MCP tools for running actions'
started: "2026-10-11"
completed: "2026-10-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: two remote-MCP tools, `list_actions` and `run_action`, over the
configured data-entry actions (`actions:` in data-entry.yaml), with exactly the
gating of `POST /api/v1/_action/{id}`. One shared operation behind both the
HTTP handler and the MCP tool.

Not in scope: built-in entity actions (`/_actions/clone`), `commands:`,
webhooks, next-action offers, stdio `rela mcp` (runs as the system principal;
it already has `lua_run`), and per-action parameters such as a measurement
period (that is the consumer's action contract via its `request:` block).

**Acceptance Criteria:**

1. `list_actions` without `id` lists the non-entity-bound actions whose
   `permission:` the caller holds, with id, label, description, confirm text and
   whether the action is request-scoped. Test: tool test with two principals,
   one holding the permission.
2. `list_actions` with `id` (`ID` or `ID@face`) lists exactly the
   `available_on` actions the detail page offers (`computeDetailActions`):
   scope, faces, `when:` and permission. Unreadable or absent entity: the same
   not-found. Test: tool test against an entity matching and not matching
   `when:`, and an unreadable one.
3. `run_action` runs the script as the signed-in principal and returns
   message, message_type and redirect; a rich response returns status,
   content_type and body. Test: tool test with a script returning each shape.
4. `run_action` refuses exactly what the HTTP route refuses, with the same
   code: unknown action, missing permission, entity-bound action without a
   readable address, action not available on the entity, config reloaded
   meanwhile. Test: table test over the shared operation plus a tool test per
   refusal.
5. An action that declares `confirm:` runs only with `confirm: true`; without
   it the tool returns the confirm text and runs nothing. Test: tool test
   asserting no write happened.
6. `request` (a JSON object) reaches `rela.request.body` only for an action
   with a `request:` block, under the same size cap; for other actions it is
   refused. Test: tool tests for both.
7. A script error returns a tool error with the correlation id and the
   degraded (non-loopback) detail. Test: tool test with a raising script.
8. Stdio `rela mcp` registers neither tool; the remote MCP registers both.
   Test: registration tests in `internal/mcp` and the remote wiring test.
   (Changed during implementation: the remote tool set is fixed at startup
   while `data-entry.yaml` reloads, so the tools are registered even when no
   action exists yet. The goldens cover only the stdio tool set, which does
   not change.)
9. Existing HTTP action tests pass unchanged.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (pattern exists in-repo: TKT-QIQO1W).

**Existing Solutions:**

- `internal/dataentry/actions.go` `handleV1Action`: the full gate sequence
  (id regex, lookup, payload cap, permission, entity resolution through the
  visibility reader, provisioning, reload refusal, detail-action gate,
  capabilities, engine call, error mapping).
- `internal/dataentry/detailactions.go`: `newDetailActionCheck`, `Allows`,
  `computeDetailActions`, `holdsActionPermission`.
- `internal/dataentry/comments_ops.go` + `comments_mcp.go` +
  `internal/mcp/tools_comments.go` (TKT-QIQO1W): the shared-operation /
  MCP-host / interface-in-Deps pattern this ticket copies.
- `internal/mcp/tools_lua.go`: `lua_run` exists but is stdio-only and runs any
  script as the system principal; not a substitute.
- No external library applies.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Extract the body of `handleV1Action` into
   `(h *writeHandler) runAction(ctx, id string, in actionInput) (*script.ActionResponse, string, *actionFailure)`
   in a new `internal/dataentry/actions_ops.go`. `actionInput` holds the
   entity address and the already-read payload. `actionFailure` carries the
   HTTP status, code and title the route writes today, and the correlation id
   / script error for script failures. The HTTP handler keeps reading the body
   (`readActionPayload`) and maps failures to the same envelopes; the
   provisioning step moves from `r.WithContext` to the ctx passed in.
2. Add `listActions(ctx, address string) ([]ActionInfo, *actionFailure)` next
   to it: without an address, the actions with `AvailableOn == nil` whose
   permission is held; with an address, the actions `computeDetailActions`
   offers, via the same `newDetailActionCheck`.
3. `internal/dataentry/actions_mcp.go`: `MCPActions` with `ListActions` and
   `RunAction`, as `MCPComments` does. `RunAction` builds the payload from the
   tool arguments: entity address, and for a request-scoped action a
   `lua.Request` whose body is the JSON object, checked against the same cap.
   Full script detail is never returned (no loopback on MCP).
4. `internal/mcp/tools_actions.go`: interface `ActionRunner` in `Deps`;
   `list_actions(id?)` and `run_action(action, id?, request?, confirm?)`.
   Registered only when `Deps.Actions` is non-nil. Confirm handling lives in
   the tool: an action with `confirm.Enabled` and no `confirm: true` returns a
   non-error result naming the confirm text and runs nothing.
5. Wiring: `MCPHost.Actions` in `internal/dataentry/mcp_http.go`, set in
   `mcpHost(a)`; `cmd/rela-server/mcp.go` assigns `deps.Actions` only when set.
   Stdio leaves it nil.
6. `.go-arch-lint.yml` only if a new import edge appears (expected: none;
   `mcp` already imports `script` and `lua`).
7. Docs in `docs-project/` (source of `docs/`): MCP server tool list and the
   actions section of the data-entry docs. Golden `tools_list` updated.

**Files to modify:**

- `internal/dataentry/actions.go` (handler becomes a thin adapter)
- `internal/dataentry/actions_ops.go` (new)
- `internal/dataentry/actions_mcp.go` (new)
- `internal/dataentry/mcp_http.go`
- `internal/mcp/server.go` (Deps field, handler set)
- `internal/mcp/tools.go` (registration)
- `internal/mcp/tools_actions.go` (new) + tests
- `cmd/rela-server/mcp.go`
- `internal/mcp/testdata/` golden tool lists
- `docs-project/` pages for MCP and actions

**Alternatives considered:**

- Call the HTTP route in-process from MCP: second auth path and an
  http.Request fabricated per call; rejected as for comments.
- Duplicate the gates in `internal/mcp`: two copies of security code drift
  (BUG-R1PQY9); rejected.
- One combined `action` tool with an `op` selector (the tool-count guidance in
  `registerTools`): listing and running have different arguments and results;
  two tools read clearer. Open to merging if preferred.
- Expose actions on stdio via the system principal: would bypass every
  `permission:` gate; rejected.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `action`: same regex `^[a-z0-9_-]{1,64}$` and config lookup.
- `id`: parsed with `entity.ParseAddress`; resolved through the visibility
  reader; unreadable and absent answer alike.
- `request`: JSON object only, size-capped with the action's cap; refused for
  actions without a `request:` block, so no new input reaches a script that
  did not opt in.
- `confirm`: boolean.

**Security-Sensitive Operations:**

- Script execution with ambient capabilities: only the action's declared
  `capabilities:` (unchanged code path).
- Writes by the script: bounded by the caller's ACL, as today.
- Error detail: always the degraded envelope.
- The shared operation guarantees the MCP tool cannot skip a gate the route
  applies.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria 1 to 9; each maps to the named
test. Integration: a real `tools/call` over the remote MCP HTTP transport with
a JWT principal runs an action that creates an entity, and the entity exists
afterwards with that principal as author.

**Edge Cases:**

- Project with no `actions:`: no tools registered.
- Action whose script is missing: tool error, no partial output.
- Entity-bound action named without `id`: not-found, as the route.
- Face-scoped action on the wrong face: not available.
- Config reload between lookup and run: `config_reloaded` refusal.
- Historical address (`@version`): not offered, refused.
- Request body at exactly the cap and one byte over.

**Negative Tests:** invalid action id, unknown action, permission missing,
unreadable entity, `when:` false, request on non-request-scoped action,
confirm missing, raising script.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Refactor of `handleV1Action` changes a status or message: existing tests
  unchanged plus a table test over the failure mapping.
- The handler relies on request-scoped state (`withProvision`, read gate on
  ctx): the operation takes ctx explicitly; the MCP ctx already carries the
  principal, ACL request and read gate (as for comments).
- Long-running scripts block the MCP call: same `lua.DefaultTimeout`.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** `docs-project/` pages that generate
`docs/mcp-server.md` and the actions section of `docs/data-entry.md`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** self-review of the plan:

- Significant: confirm-protected actions (atlas: "concept wordt
  overschreven") would run silently from an agent. Addressed: `confirm: true`
  is required (AC 5).
- Significant: `request` passthrough could hand body data to scripts that never
  opted in. Addressed: refused unless the action declares `request:` (AC 6).
- Minor: listing global actions could reveal action ids the caller may not
  run. Addressed: filtered on `permission:` (AC 1); ids are config, not
  secrets, per the existing 403 message.

## Approval

Approved by Jeroen on 2026-10-11: two tools (`list_actions`, `run_action`),
the gates of `handleV1Action` move to a shared operation in this PR, and the
existing HTTP tests must pass unchanged. Out of scope: the AAL2 measurement
quarter in atlas (a periodic task covers it).
