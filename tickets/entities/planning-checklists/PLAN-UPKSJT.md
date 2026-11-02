---
id: PLAN-UPKSJT
type: planning-checklist
title: 'Planning: Serve the effective in: on message type views'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Since TKT-S5OJ7S, `session.default_in: keyed` makes the load give a message type
without `in:` the states whose `on:` keys it (`messageInCheck.withSession` in
`internal/realtimecfg/message_in.go`). The result lands in `MessageType.In`, and
the unexported `MessageType.derived` records that it was derived.
`realtime.yaml` no longer shows the effective list, and no read surface serves
it: an operator or agent can only learn it from a 409 `session_wrong_state`.
This ticket serves the effective `in:`, and whether the load derived it, on
every message type view, next to `decision_button`.

**Scope:**

In scope:

- An exported accessor on `realtimecfg.MessageType` for the derived flag.
- Two new fields, `in` and `in_derived`, on:
  - `realtime.MessageTypeDescription` (feeds REST `GET /api/v1/_schema/channels`);
  - `realtime.MessageTypeView` (feeds MCP `read_channel`, and `rela channel read` through the same struct);
  - `v1.ChannelMessageTypeSchema` (the REST wire type).
- OpenAPI: the two properties on the message type in the `ChannelSchema` component.
- MCP: the `read_channel` tool description names the fields; regenerate `tools_list_channels.golden.json`.
- Frontend: carry the two fields on `ChannelMessageTypeSchema` in `frontend/src/types/channels.ts` and its `wireKeys` list. The SPA does not consume them.
- Docs: `docs-project/entities/guides/GUIDE-realtime.md`.

Out of scope:

- Any change to how `in:` is derived or enforced.
- SPA behaviour. The composer already narrows by `_actions.post` and `can_post`, which apply `in:`; a second source would only duplicate that rule.
- The REST message page (`v1.MessagePage`). It carries no message types; REST clients get types from `_schema/channels`.
- Printing `in:` in `rela channel read` (`writeMessageType` in `internal/cli/channel_render.go`). It is a possible follow-up; the CLI `-o json` output gains the fields for free because it marshals `MessageTypeView`.
- The `in_key` path (`ChannelType.InKey`) on the wire. The bool says the same thing in a shorter form.

**Wire shape (decided):**

| Field | JSON | Go type | omitempty |
| --- | --- | --- | --- |
| `In` | `in` | `[]string` | yes |
| `InDerived` | `in_derived` | `bool` | yes |

| Case | `in` | `in_derived` |
| --- | --- | --- |
| Channel type without a session | absent | absent |
| No `in:`, and not derived (no `default_in`, or no state keys the type) | absent: every state | absent |
| `in:` written in the file (also with `default_in: keyed`) | the list as written | absent |
| Derived from `session.default_in: keyed` | the keyed states, in `session.states` order | `true` |

- Absent `in` keeps today's meaning: postable in every state. An empty list never reaches the wire, because the load refuses `in: []`.
- `in_derived` is never `true` without `in`.
- Both fields are config, the same for every reader. Per CLAUDE.md, the configuration is not a secret: the state names are already served in `session.states`.
- The order of `in` carries no meaning. A written list keeps its written order and a derived list follows `session.states`; the v1 doc comment (which the TS comment points to), the OpenAPI description and the guide say "order carries no meaning" so no client depends on either. Normalizing a written list was rejected: it would make the served list differ from the file.
- `in` explains the rule; it is not the gate. `_actions.post` and `can_post` stay authoritative for whether a caller may post now. The `ChannelTypeView` godoc, the v1 doc comment (which the TS comment points to) and the OpenAPI description say so, so no client computes postability from a cached descriptor.
- A message type belongs to one channel type, so to at most one session. A type cannot have two `in` lists from two sessions.

**Acceptance Criteria:**

1. `realtimecfg.MessageType.InDerived()` returns true exactly for a type whose `In` the load derived from `session.default_in: keyed`. It returns false for a written `in:`, for a type without `in:`, and for a `MessageType` built by hand. Test: extend `TestDefaultIn_Derive` and `TestDefaultIn_AbsentKeepsToday` in `internal/realtimecfg/default_in_test.go`.
2. `Service.Describe()` serves `In` and `InDerived` on each `MessageTypeDescription`, and `Service.Read(... Schemas: true)` serves the same values on each `MessageTypeView`. Test: new `TestDescribe_In` in `internal/realtime/describe_test.go` over the `pocRealtimeDefaultIn` fixture, with one written `in:`, one derived and one unkeyed type, asserting Describe and Read agree.
3. `GET /api/v1/_schema/channels` serves `in` and `in_derived` per the table above: derived gives `"in": [...]` with `"in_derived": true`; written gives `"in"` without `in_derived`; no `in:` gives neither key. Test: new `TestChannelSchema_In` in `internal/dataentry/channels_schema_test.go` over the `defaultInRealtime` fixture (see Test Plan).
4. The answer of AC 3 is the same for every principal. Test: `TestChannelSchema_In` compares the bodies for two principals with different roles.
5. MCP `read_channel` serves `in` and `in_derived` on each `message_types` entry with the same rules, and its description says what they mean. The `post_message` description points to `read_channel`'s `message_types[].in`. Test: new `TestReadChannel_In` in `internal/mcp/tools_channels_test.go` over the `defaultInYAML` fixture (see Test Plan); `TestGolden_ChannelToolsList` passes after `UPDATE_GOLDEN=1`.
6. The OpenAPI message type schema lists `in` (array of string) and `in_derived` (boolean), and its key set equals the JSON names of `v1.ChannelMessageTypeSchema`. Test: new key-set check in `internal/openapi/channels_test.go` using the existing `schemaKeys`/`jsonNames` helpers; the OpenAPI goldens under `internal/dataentry/testdata/golden/*/openapi.authenticated.golden` are regenerated.
7. The SPA's `ChannelMessageTypeSchema` carries `in?: string[]` and `in_derived?: boolean`, and its `wireKeys` list matches the Go field set. Test: `TestChannelWireGolden -update` rewrites `channel_wire_fields.json`; `frontend/src/types/wireFields.test.ts` passes.
8. GUIDE-realtime.md documents both fields under `_schema/channels` and in the `default_in` section.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small additive field, approach is fixed by the decision_button precedent)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: plain struct fields on existing views)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal wire shape)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `decision_button` (TKT-TXZZZ2) took the same path through every layer: `realtimecfg.MessageType.DecisionButton` to `realtime.MessageTypeDescription.DecisionButton` (`describeMessageType`, `internal/realtime/describe.go`) and `realtime.MessageTypeView.DecisionButton` (`call.messageTypes`, `internal/realtime/read.go`), then `wireMessageTypeSchema` (`internal/dataentry/channels_schema.go`) to `v1.ChannelMessageTypeSchema`, `decisionButtonSchema` in `internal/openapi/channels.go`, `buttonOptionsNote` in `internal/mcp/tools_channels.go`, and the TS mirror. Its tests are the templates: `TestDescribe_DecisionButton`, `TestChannelSchema_DecisionButton`, `TestOpenAPI_ChannelButtonOptions`.
- `ChannelType.InKey` (`internal/realtimecfg/compiled.go`) already reads `mt.derived`; the accessor follows the same rule.
- `MessageType.clone` (`internal/realtimecfg/clone.go`) copies `derived` through `out := *mt` and clones `In`, so a reload keeps both.
- `pocRealtimeDefaultIn` in `internal/realtime/session_affordance_test.go` is a ready fixture with derived types.
- `ChannelSessionStateSchema.WaitingFor` (`[]string`, `omitempty`) is the precedent for an optional list of state-related names on the schema wire.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `internal/realtimecfg/compiled.go`: add
`func (mt *MessageType) InDerived() bool { return mt.derived }` with a doc
comment ("reports that In was derived from session.default_in; false for a
written in:, for no in:, and for a MessageType built by hand"). Keep the field
unexported so only the loader sets it. Let `InKey` call it. Add
`[MessageType.InDerived]` to the `In` godoc and the `InKey` godoc.
2. `internal/realtime/describe.go`: add to `MessageTypeDescription`
`In []string` and `InDerived bool` after `DecisionButton`. Their comment links
`[realtimecfg.MessageType.In]` and `[realtimecfg.MessageType.InDerived]` instead
of restating that nil means every state. Fill them in `describeMessageType` with
`slices.Clone(mt.In)` and `mt.InDerived()`. Update the `ChannelTypeView` godoc:
it carries no gate, role or condition except `in`, which explains the state
rule; the affordances on each channel (`_actions.post`, `can_post`) stay
authoritative.
3. `internal/realtime/read.go`: add to `MessageTypeView`
`In []string \`json:"in,omitempty"\``and`InDerived bool
\`json:"in_derived,omitempty"\``after`DecisionButton`, with the same links as
step 2. Fill them in `call.messageTypes`under the existing "Config, the same for
every reader" comment. Clone`In`with`slices.Clone`so the view does not alias the
config snapshot; add the`slices`import, which`read.go` does not have yet.
4. `internal/apiwire/v1/channels.go`: add the same two fields with the same
tags to `ChannelMessageTypeSchema`, and extend its doc comment with one
sentence: `In` lists the states the type may be posted in (absent: every state;
order carries no meaning), `InDerived` says it came from `session.default_in`,
and `_actions.post`/`can_post` stay authoritative. `apiwire/v1` does not import
`realtimecfg`, so it cannot link; this is the one wire-side statement, and the
TS comment points to it.
5. `internal/dataentry/channels_schema.go`: copy both in
`wireMessageTypeSchema`.
6. `internal/openapi/channels.go`: in `addChannelDescriptorSchema`, add to
the `messageType` properties `"in": {Type: "array", Items: str(), Description:
"The session states in which the type may be posted, from in: or derived from
session.default_in; absent when every state allows it. The order carries no
meaning. Explains the rule; _actions.post and can_post on the channel stay
authoritative"}` and `"in_derived": {Type: "boolean", Description: "true when in
was derived from session.default_in: keyed; absent otherwise"}`. Neither is
required.
7. `internal/mcp/tools_channels.go`: extend the `read_channel` description
sentence "A message type may be limited to some session states; ..." with
"message_types[].in lists them, and in_derived is true when they come from
session.default_in". Extend the same sentence in the `post_message` description
with "read_channel's message_types[].in lists them", because an agent refused
with `session_wrong_state` reads `post_message` first. Regenerate
`testdata/tools_list_channels.golden.json`.
8. `frontend/src/types/channels.ts`: add `in?: string[]` and
`in_derived?: boolean` to `ChannelMessageTypeSchema`, with one comment that
points to the Go `v1.ChannelMessageTypeSchema` doc instead of restating it, and
add `'in'`, `'in_derived'` to its `wireKeys` list.
9. Regenerate goldens: `go test ./internal/dataentry -run TestChannelWireGolden -update`
and the OpenAPI goldens. `channel_wire_fields.json` gains `in` and `in_derived`.
`channel_wire.json` does not change: the `session_schema` sample uses
`breachRealtime`, whose only `in:` is on a reminder, not on a message type. That
is enough, because `wireFields.test.ts` flags only unknown keys and takes the
field set from the Go struct. The wire golden fixture stays as it is.
10. Docs: edit `docs-project/entities/guides/GUIDE-realtime.md` (never
`docs/`), then `just docs`.

**Files to modify:**

- `internal/realtimecfg/compiled.go`
- `internal/realtimecfg/default_in_test.go`
- `internal/realtime/describe.go`
- `internal/realtime/read.go`
- `internal/realtime/describe_test.go`
- `internal/apiwire/v1/channels.go`
- `internal/dataentry/channels_schema.go`
- `internal/dataentry/channels_schema_test.go`
- `internal/dataentry/testdata/channel_wire_fields.json` (regenerated)
- `internal/dataentry/testdata/golden/*/openapi.authenticated.golden` (regenerated)
- `internal/openapi/channels.go`
- `internal/openapi/channels_test.go`
- `internal/mcp/tools_channels.go`
- `internal/mcp/tools_channels_test.go`
- `internal/mcp/testdata/tools_list_channels.golden.json` (regenerated)
- `frontend/src/types/channels.ts`
- `docs-project/entities/guides/GUIDE-realtime.md` (and the generated `docs/` copy via `just docs`)

**Alternatives considered:**

- `in_source: "in" | "default_in"` instead of a bool. Rejected: two values, so a bool says the same with less to validate, and an absent `in` would need a third value or a rule.
- `in_key` with the key path, as `ChannelType.InKey` returns. Rejected: the path is a debugging aid already in the 409 detail; a client needs only the yes/no.
- Serving `in: []` for "every state" and dropping omitempty. Rejected: `[]` reads as "no state", the opposite meaning, and it changes every existing answer.
- Exporting the `derived` field. Rejected: anyone could then set it; a read-only method keeps the loader the only writer.
- Serving only on the REST schema. Rejected: the ticket names MCP `read_channel`, and an agent sees types only through it.

**Dependencies:** none new. Uses `slices.Clone`; `describe.go` already imports
`slices`, `read.go` gains the import.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- The only source is `realtime.yaml`, validated at load. Every name in `In` is a declared state (`messageInCheck.one`, `keyedStates`). No request input reaches the new fields.

**Security-Sensitive Operations:**

- None. The fields are config, and config is not a secret (CLAUDE.md, "The configuration is not a secret; the data is"). State names are already on the wire in `session.states`. No per-principal filtering is added, and none is needed. Who may post stays in `_actions.post` and `can_post`, which are per principal and unchanged. Read-path gating of channels is unchanged: `read_channel` already refuses a caller without read access before it builds message types.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestDefaultIn_Derive`: per case, assert `mt.InDerived() == (wantKey == defaultInKey)`. `TestDefaultIn_AbsentKeepsToday`: `InDerived()` false for a written `in:` and for `&MessageType{}`. |
| 2 | `TestDescribe_In` (`internal/realtime/describe_test.go`): `pocRealtimeDefaultIn` fixture; compare `Describe()` and `Read(Schemas: true)` for a derived, a written and an unkeyed type. |
| 3 | `TestChannelSchema_In` (`internal/dataentry/channels_schema_test.go`): real HTTP GET over `defaultInRealtime`, decode raw JSON, assert key presence per row of the table. |
| 4 | `TestChannelSchema_In` asserts byte-equal bodies for two principals with different roles. |
| 5 | `TestReadChannel_In` (`internal/mcp/tools_channels_test.go`): `read_channel` over `defaultInYAML`, called without a cursor so `message_types` is served (`schemas` defaults to true then); assert `in`/`in_derived` in the JSON for the breach and room types. `TestGolden_ChannelToolsList`. |
| 6 | `TestOpenAPI_ChannelMessageTypeSchema` in `internal/openapi/channels_test.go`: `schemaKeys(messageType)` equals `jsonNames(v1.ChannelMessageTypeSchema)`. OpenAPI golden tests. |
| 7 | `TestChannelWireGolden` plus `wireFields.test.ts` (`npm run test:run` in `frontend/`). |
| 8 | Review; `just docs` regenerates `docs/` without a diff against the guide. |

Integration: AC 3 and 5 go through the real HTTP handler and the real MCP
handler with a loaded `realtime.yaml`, not through hand-built views.

**Fixtures:**

`pocRealtimeDefaultIn` lives in package `realtime`, so `dataentry` and `mcp`
cannot use it. Each test package derives its fixture from its own breach example
with `strings.Replace`, as `timerYAML` already does:

- `defaultInRealtime` in `internal/dataentry/channels_schema_test.go`: `breachRealtime` with `default_in: keyed` added under `session:` and `in: [detected, contained]` added to `text`. Rows: `contained`, `dismiss`, `escalated` derived `[detected]`; `assessed` derived `[contained]`; `notified` derived `[assessed]`; `text` written; `reminder` unkeyed (no `in`); `plain.text` without a session.
- `defaultInYAML` in `internal/mcp/tools_channels_test.go`: `breachYAML` with `default_in: keyed` added under `session:` and `in: [detected]` added to `text`. Rows: `contained`, `dismiss`, `escalated` derived; `text` written; the `room` channel type without a session.

Each test fails first if its `strings.Replace` changed nothing, as `useTimers`
does.

**Edge cases:**

- Channel type without a session: no `in`, no `in_derived`.
- `default_in: keyed` with a type that no state keys (text, a reminder post type, a choices answer type): no `in`, no `in_derived`.
- `default_in: keyed` with a written `in:` that widens the keyed states: `in` as written, no `in_derived`.
- A type keyed in two states: `in` in `session.states` order, not `on:` order.
- Reload that adds or drops `default_in`: covered by construction, not by a new test. `Describe()` and `Read` each load one snapshot, `MessageType.clone` copies `derived`, and the guide's refresh event makes clients reread `_schema/channels`.
- Several sessions: cannot occur. A message type belongs to one channel type, so to at most one session.
- Mutating the returned `In` must not change the config: covered by cloning. `TestDescribe_In` sets `In[0] = "x"` on the returned `MessageTypeDescription` and on the returned `MessageTypeView`, then calls `Describe()` and `Read` again and asserts the original value. Appending would not prove this, because `append` never writes into elements the config's slice can see.

**Negative Tests:**

- `in_derived` is never present without `in` (asserted in `TestChannelSchema_In` over every type).
- `in` is never an empty array on the wire (asserted in the same loop).
- A principal without channel read access still gets the same `_schema/channels` body (AC 4) and still gets a 404/denial from `read_channel` (existing tests, unchanged).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Missing one of the parallel mirrors (Go wire, OpenAPI, TS, goldens). Mitigation: the existing key-set tests (`wireFields.test.ts`, `jsonNames` in OpenAPI tests) fail on a missing field; AC 6 adds the one that is missing for the message type schema.
- Aliasing `mt.In` from the shared config snapshot into a view that a caller mutates. Mitigation: `slices.Clone` in both builders and the element-mutation check in `TestDescribe_In`.
- A client treats `in` on a cached descriptor as the gate. Mitigation: every surface says `_actions.post` and `can_post` stay authoritative (Wire shape notes).
- Older clients: the fields are additive and optional; the SPA ignores them and decodes leniently. No version gate needed.
- `in` is a reserved word in some client languages (for example a Python attribute). JSON key access is unaffected; the name mirrors the YAML key, which is the point.
- Plimsoll: one new method on `realtimecfg.MessageType`; check `just plimsoll` stays under the cap.

Security risk: none beyond the above; the data is config.

Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- `docs-project/entities/guides/GUIDE-realtime.md`:
  - The `GET /api/v1/_schema/channels` paragraph: "A message type limited to some session states has `"in"` with those states, and `"in_derived": true` when they come from `session.default_in`. The order carries no meaning. Whether you may post now is in the channel's `_actions.post` and `can_post`."
  - The "Take `in:` from the transitions" list: one bullet saying the derived list is visible in `_schema/channels` and in `read_channel`'s `message_types`.
  - Regenerate `docs/` with `just docs`; do not edit `docs/` directly.
- `internal/mcp/tools_channels.go`: the `read_channel` and `post_message` tool descriptions (served to agents).
- OpenAPI descriptions in `internal/openapi/channels.go`.
- `docs/metamodel.md`, `docs/cli-reference.md`, `docs/data-entry.md`, CLAUDE.md, README.md: no change.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 10 findings (4 minor, 6 nits), no critical or
significant. All 10 are folded into the plan above:

1. Vacuous aliasing test: `TestDescribe_In` mutates an element on both views, then rereads.
2. `ChannelTypeView` godoc: updated in step 2; every surface says `_actions.post` and `can_post` stay authoritative.
3. Fixtures: `defaultInRealtime` and `defaultInYAML` named under Test Plan; the MCP call passes no cursor. The finding said `breachRealtime` has a written message-type `in:`; it does not (its `in:` is on a reminder), so the fixture adds one.
4. `post_message` description: points to `read_channel`'s `message_types[].in` (step 7).
5. Step 9: resolved; fixture unchanged and `channels_wire_golden_test.go` dropped from the files. The finding said `channel_wire.json` gains `in`; it does not, since no message type in the sample has `in:`.
6. AC 4: one test option.
7. `slices` import: `read.go` gains it.
8. Duplicated prose: Go views link `realtimecfg`; v1 holds the one wire-side sentence; TS points to v1.
9. Order of `in`: documented as carrying no meaning.
10. Several sessions and reload: stated as impossible and as covered by construction.
