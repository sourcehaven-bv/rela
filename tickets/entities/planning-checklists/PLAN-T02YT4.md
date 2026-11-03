---
id: PLAN-T02YT4
type: planning-checklist
title: 'Planning: Realtime 3/6: channel MCP tools, watch and CLI'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- MCP tools `list_channels`, `read_channel`, `post_message`, `edit_message`, `remove_message`, `create_channel` on both transports, registered when realtime is on.
- A per-message-type props JSON Schema in `read_channel`, shared with OpenAPI.
- An untrusted-content notice in results, descriptions and server instructions.
- `read_channel` as a `watch` target: channel wake plus a cross-process poll.
- New service seams: `Service.Read` (one capture, cursor rule), `realtime.UserText`, `PostInput.LogOnly`.
- `rela channel list|read|post|edit|remove|create|watch` with `-o json`.
- A global `--run-as` / `RELA_RUN_AS` identity for the local CLI and stdio `rela mcp`, with a self-asserted marker (`Principal.AssertedBy`, `Author.Asserted`). rela-server refuses it.
- Stdio `rela mcp` hot reload of realtime.yaml.
- Docs.

Out of scope:
- Lifecycle and actions tools and verbs (T4: `run_message_action`, `channel_lifecycle`, `rela channel close|reopen|archive|delete|action`).
- Triggers (T5) and the SPA (T7).
- Server-side cross-process live frames for CLI and stdio posts (phase 5 or a follow-up; decision 8).
- MCP resources and prompts for channels.
- Lua access to channels.
- Cross-node signals.

**Acceptance Criteria:**
1. Registration: realtime on gives the 6 tools with the documented schemas on both transports. Realtime off gives no tools and an unchanged default golden. A reload to off gives "channels are not enabled". Test: `TestChannelTools_RegisteredOnlyWhenEnabled`, `TestToolsListGolden_Channels`.
2. One service: MCP and CLI use only `*realtime.Service`; there is no `channels.Store` in mcp or cli. Test: a grep lint test plus arch-lint.
3. Uniform not-found: the error text for a hidden or missing anchor equals `show_entity` / `rela show`. An unknown type gives `unknown_channel_type`. A read denial names the rule. Test: `TestChannelTools_HiddenAnchorTextEqualsShowEntity`, `TestChannelCLI_HiddenAnchorTextEqualsShow`.
4. Restricted refs render `{restricted:true}`. A hidden ref and a missing ref give the same `invalid_ref` text. Test: `TestChannelTools_RestrictedRef`, `TestChannelTools_InvalidRefUniform`.
5. Per-type schemas compile and agree with `validateProps`, and OpenAPI uses the same generator. Test: `TestPropsSchema_Compiles`, `TestPropsSchema_AgreesWithValidate`, `TestChannelPaths`.
6. The untrusted notice is the first key of every content result and is in the descriptions and instructions. CLI table output escapes control characters and indents. Test: `TestChannelResults_NoticeFirst`, `TestChannelRender_ControlCharsAndIndent`.
7. A watch on `read_channel` returns on a post, both in-process and cross-instance within the poll. Test: `TestWatch_ReadChannelReturnsOnPost`, `TestWatch_ReadChannelCrossInstancePoll`.
8. A watch does not reveal hidden activity: a hidden anchor, a read-denied type or a hidden `many` channel times out unchanged. Test: `TestWatch_ReadChannelHiddenAnchorTimesOut` and siblings, `TestReadChannel_Deterministic`.
9. Watch bounds: caps of 16, 4 and 32; the poll only with channel targets and only for them; timeout at most 50s. Test: `TestWatch_ChannelTargetsCountTowardCaps`, `TestWatch_PollReplaysOnlyChannelTargets`.
10. `Read` captures once, and its budget is flat at 10 and 50 messages. Test: `TestRead_OneAccessOneSnapshot`, `TestRead_Budget10vs50`.
11. The cursor rule loses nothing under concurrency, and HTTP uses it too. Test: `TestRead_CursorRule`, `TestRead_AfterRevNoLossUnderConcurrency`, `TestWirePage_UsesRealtimeCursors`.
12. Signals: remote is accepted; stdio and CLI give `signals_not_kept` after authz. Rate limits answer with a retry time. Test: `TestPost_LogOnlyRefusesSignalAfterAuthz`, `TestPostMessage_StdioSignalRefused`, `TestChannelTools_RateLimitedText`.
13. Identity: default `$USER`; `--run-as` validated, with no claims and `AssertedBy` set; `asserted` rendered; `asserted_by` audited; rela-server refuses the env var. Test: `TestLocalPrincipal_*`, `TestRunAs_AssertedRecorded`, `TestServer_RefusesRunAsEnv`.
14. CLI commands and JSON shapes, watch timeout, follow and exit codes, `-P` coercion. Test: `TestChannelCLI_*`, `TestChannelWatchCLI_*`.
15. Every realtime denial type has `UserText`. Test: `TestUserText_CoversEveryType`.
16. Stdio realtime.yaml reload. Test: `TestMCPStdio_RealtimeReload`.
17. Plimsoll and arch-lint pass. Test: `just lint`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. Effort m, and the design reuses existing in-repo
patterns. Prior art is in .ignored/phase3-plan.md and .ignored/t3-plan.md.

**Existing Solutions:**
- Libraries:
  - `github.com/modelcontextprotocol/go-sdk` (already used; no push on stateless HTTP, hence long-poll watch).
  - `github.com/google/jsonschema-go` (already an indirect dependency via the go-sdk), used in tests to compile and validate the props schemas. No new dependency at runtime.
- Codebase patterns:
  - `internal/mcp/tools_lease.go`: an optional service dependency, tools registered when it is present, a "feature off" answer after a reload, shared `lease.UserText`.
  - `internal/mcp/tools_watch.go`: the replay-digest watch, wake sources, caps, fixed tick.
  - `internal/mcp/tools_comments.go`: a `Deps` sub-struct with reader and writer split, stdio wiring differences.
  - `internal/cli/lease.go`: kong verb family, `-o json`, error text sharing.
  - `internal/dataentry/channels_handler.go` and `channels_errors.go`: T3b address parsing, error table, completeness test.
  - `internal/openapi/channels.go` `postBodySchema` / `messagePropSchema`: the source of the shared props schema.
  - `internal/cli/db_importfs.go` `printable`: control-character escaping.
  - `principal.ValidateRunAs` with scheduler `anySystemIdentity`: run-as validation.
  - `dataentry/realtime_wiring.go` `watchRealtime`: realtime.yaml reload.
- Reference implementations: MCP long-poll and cursor patterns follow rela's own `watch` (TKT-FMS8BS); `after_rev` paging follows T3b.
- rela concepts:
  - "The configuration is not a secret; the data is": type and rule names are shown, entity existence is uniform.
  - DEC-ZBI39P: gating is a wiring decision.
  - Phase 3 decision 2: local identity by flag or env.
  - The T6 pins: the flag is local only, never sets principal_type, and is marked self-asserted.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- See .ignored/t6-plan.md sections 0-5.
- New service seams:
  - `Service.Read(ctx, RawAddress, ReadRequest) (ReadView, error)`: one `open`, one `Messages`, one ref batch; cursor rule: after_rev uses `HasMore ? last.Rev : max(last.Rev, req.AfterRev, channel.Rev)`, and the newest page sets `next_after_rev = channel.Rev`.
  - `realtime.UserText(err)`, `realtime.UntrustedNotice`.
  - `PostInput.LogOnly` giving 422 `signals_not_kept` after the post-role check.
  - `realtimecfg.MessageType.PropsSchema()`.
- MCP:
  - `Deps.Channels ChannelDeps{Service ChannelService; RefuseSignals bool}`, `handlerSet.channels`.
  - Tools registered before the watch handler when the service is enabled.
  - Results are one JSON object with `notice` first.
  - Errors go through `UserText`, with anchor misses through `entityReadFailed`.
- Watch:
  - `read_channel` is added to the dynamic watchable list.
  - Wake on `Service.Subscribe()`.
  - A `WatchDeps.ChannelPoll` ticker (stdio 2s, remote 5s) marks only channel targets dirty.
  - Replays stay gated, fixed-clock and digest-compared.
- CLI:
  - Kong `ChannelCmd` binding `*channelServices`.
  - `-P` coercion by declared type.
  - The watch loop does a fresh `Read` per tick, with in-process wake plus `--poll`, and `--timeout` / `--follow`.
  - Table rendering escapes and indents untrusted text.
- Identity: `localPrincipal(tool, runAs)` in `runKong` and `McpCmd.Run`; `Principal.AssertedBy`; rela-server refuses `RELA_RUN_AS`.

Alternatives rejected:
- MCP notifications or resource subscriptions instead of watch: no push on stateless HTTP, and no ad-hoc notifications in the go-sdk.
- A `wait` argument on `read_channel`: it would duplicate watch caps and digest safety.
- Calling Head and then Messages from the transports: two captures, inconsistent cursors.
- Raw `Store.Head` polling in the watch: more code and an ungated read path. The replay poll is simpler, and a head pre-check is a follow-up.
- Wrapping untrusted text in XML-like tags: JSON string boundaries cannot be broken by content, and tags can be.
- `--as` / `RELA_PRINCIPAL`: collides with `rela acl map --as` (principal type) and `RELA_PRINCIPAL_HEADER`.
- `RawUser` as the self-asserted marker: it changes `QueryIdentity` resolution (is_current_user would match the OS user).

**Files to modify:**
- `internal/realtime/{read.go,usertext.go,service.go,errors.go,authz.go,render.go}` (+tests)
- `internal/realtimecfg/jsonschema.go` (+test)
- `internal/openapi/channels.go` (+test)
- `internal/channels/channels.go`
- `internal/principal/{principal.go,sharekey.go}` (+tests)
- `internal/dataentry/{channels_wire.go,channels_errors.go}`
- `internal/mcp/{tools_channels.go,tools_watch.go,server.go,tools.go}` (+tests, new golden)
- `internal/cli/{mcp_wiring.go,mcp.go,channel.go,channel_render.go,channel_watch.go,identity.go,printable.go,kong.go,cli_wiring.go}` (+tests)
- `internal/dataentrywire/mcp.go`
- `cmd/rela-server/main.go` (+test)
- `.go-arch-lint.yml`
- docs: `docs/mcp-server.md`, `docs/cli-reference.md`, `docs/realtime.md`, `docs/acl-security.md`, docs-project guides

Dependencies:
- `*realtime.Service` (T3a), `realtime.Subscription`, `realtimecfg.Compiled` and `MessageType`.
- `appbuild.ChannelService` and `appbuild.Realtime(svc).Reload`.
- `principal.ValidateRunAs`.
- go-sdk; `google/jsonschema-go` (tests only, becomes a direct dependency).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- MCP tool arguments:
  - Unknown arguments are refused by `addTool`.
  - Address strings are parsed by the service (`entity.ParseAddress`; channel type by the realtimecfg regex; a `many` name is looked up only after the read check).
  - `seq`, `reply_to`, `limit` and cursors must be positive whole numbers in range; otherwise a tool error.
  - `props` is a free JSON object, validated by the service against the message type (allowlist of declared properties, types, enum, max, 32-prop and 16 KiB caps).
  - `reason` is capped at 500 runes by the store.
- Watch: targets are validated by `parseTargets` (allowlisted tools, an object `args`). The cursor is HMAC-keyed and capped at 4 KiB.
- CLI flags:
  - `-P key=value` is coerced only to the declared type.
  - `--props-json` must be a JSON object (`@file` / `-` read with a size cap of `MaxMessageBytes` + 1 KiB).
  - `--poll` is 1s to 60s and `--timeout` at most 1h.
  - `--run-as` / `RELA_RUN_AS` goes through `principal.ValidateRunAs` (no space or control characters; integration-name grammar; `unknown` refused). An invalid value exits 1 before any service call.
- Channel content returned to agents (untrusted): delivered only as JSON string values with a fixed leading notice. CLI table output replaces control characters and prefixes lines.
- Config (realtime.yaml): operator-trusted. Labels may appear in schemas and denials.

**Security-Sensitive Operations:**
- Authorization: every operation goes through `realtime.Service` (anchor read through the gated reader, then read role, then operation gate), with one ACL request per operation. A watch replay rebinds access every pass.
- Existence oracles:
  - The anchor-miss text equals `show_entity` / `rela show`.
  - A hidden ref and a missing ref give one text.
  - The name lookup comes after the read check.
  - The watch digest is unchanged by hidden activity (fixed clock, gated replay, deterministic result).
- Identity assertion: `--run-as` is local only. rela-server refuses the env var at startup, and remote MCP takes identity only from the verified JWT. It never sets verified claims, `AssertedBy` records who asserted it, and docs state that it inherits that name's acl.yaml grants under local full trust.
- Prompt injection: the notice in results, descriptions and instructions; no channel data in tool schemas.
- Resource bounds:
  - Watch caps 16, 4 and 32, a poll period of at least 2s, the per-identity replay slot, 4 parallel replays.
  - MCP page max 50; rate buckets per (user, org).
  - CLI watch timeout at most 1h, except `--follow`, which is interactive and local.

Error handling:
- Denial titles never name entities or messages beyond what was addressed.
- Unexpected errors are logged with user and tool only (never props, reason or ids from refs) and answered "channel request failed".
- Removal reasons and removers are shown only to remove-gate holders (service rule).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: `TestChannelTools_RegisteredOnlyWhenEnabled`, `TestChannelTools_OffAfterReload`, `TestToolsListGolden_Channels`
- AC2: grep lint tests in mcp and cli, plus arch-lint
- AC3: `TestChannelTools_HiddenAnchorTextEqualsShowEntity`, `TestChannelCLI_HiddenAnchorTextEqualsShow`, `TestChannelTools_ReadDeniedNamesRule`, `TestChannelTools_UnknownType`
- AC4: `TestChannelTools_RestrictedRef`, `TestChannelTools_InvalidRefUniform`
- AC5: `TestPropsSchema_Compiles`, `TestPropsSchema_AgreesWithValidate`, `TestChannelPaths`
- AC6: `TestChannelResults_NoticeFirst`, `TestChannelToolDescriptions_Notice`, `TestServerInstructions_Channels`, `TestChannelRender_ControlCharsAndIndent`
- AC7: `TestWatch_ReadChannelReturnsOnPost`, `TestWatch_ReadChannelCrossInstancePoll`
- AC8: `TestWatch_ReadChannelHiddenAnchorTimesOut`, `..._ReadDeniedTimesOut`, `..._ManyChannelHiddenTimesOut`, `TestReadChannel_Deterministic`
- AC9: `TestWatch_ChannelTargetsCountTowardCaps`, `TestWatch_PollReplaysOnlyChannelTargets`, `TestWatch_NoPollWithoutChannelTarget`
- AC10: `TestRead_OneAccessOneSnapshot`, `TestRead_Budget10vs50`
- AC11: `TestRead_CursorRule`, `TestRead_AfterRevNoLossUnderConcurrency`, `TestWirePage_UsesRealtimeCursors`
- AC12: `TestPost_LogOnlyRefusesSignalAfterAuthz`, `TestPostMessage_RemoteSignal`, `TestPostMessage_StdioSignalRefused`, `TestChannelTools_RateLimitedText`
- AC13: `TestLocalPrincipal_*`, `TestRunAs_AssertedRecorded`, `TestAuditAssertedBy`, `TestServer_RefusesRunAsEnv`, `TestPrincipal_AssertedByInEqualAndShareKeyNotIdentity`
- AC14: `TestChannelCLI_*`, `TestChannelWatchCLI_TimeoutExit0`, `..._FollowStopsOnCancel`, `..._ErrorReturnsAtOnce`, `..._PollBounds`
- AC15: `TestUserText_CoversEveryType`, `TestChannelErrorStatus_CoversEveryType`
- AC16: `TestMCPStdio_RealtimeReload`
- AC17: `just lint`

Integration approach:
- MCP tests run a real `Server` over an `appbuild` test assembly with a realtime.yaml fixture, a declarative acl.yaml (reader, poster, outsider), and fs and sqlite channel stores.
- The cross-instance watch test builds two services over one store with separate `Local`s.
- CLI tests run the kong commands through the existing CLI fixture (`newCLIBundles`) with `-o json`.
- The budget test uses `storetest.Counting`.

**Edge Cases:**
- A `cardinality: one` channel with no messages: `Read` returns an empty open channel with `next_after_rev` 0, and the first post wakes a watch.
- A `many` channel with a `name` on a `one` type, and none on a `many` type: service errors pass through.
- A face-anchored type addressed by bare id: `face_required`.
- `ID@face` on a family type: anchors the family.
- Cursors: two cursors set gives `invalid_message`. 0 and negative are refused at the transport. `limit` above max is clamped (MCP 50, CLI 200).
- A full `after_rev` page: the next cursor is `last.Rev`, not `channel.rev`.
- A message type removed by a config reload: messages render without props, and a post gives `invalid_message`.
- A removed message: placeholder only, with reason visible to moderators only.
- Props: unicode, empty strings, null (treated as absent), 16 KiB plus 1 byte (`too_large`), 33 props (`too_many_props`), a wrong type for integer from `-P` (CLI coercion error before the service).
- Untrusted text with ESC sequences, CR, NUL, bidi overrides and a forged header line: escaped and indented in CLI table output, verbatim in JSON.
- A watch where every target is a channel, and a mix: the poll replays only channel targets. A target becoming hidden mid-watch reports one change (revocation), then stays quiet.
- A watch whose cursor came from another process: every target reported changed once (existing behaviour).
- A concurrent post during `Read`: duplicate allowed, loss not allowed.
- `$USER` unset: the user is `unknown`, so writes give `no_principal`.
- `--run-as` with surrounding space, a control character, `integration:BAD`, or `unknown`: refused.
- `RELA_RUN_AS` set on rela-server: startup error.
- A reload turning realtime off mid-watch: replays answer "not enabled" (one change), then stay quiet.

**Negative Tests:**
- Hidden anchor, missing anchor, unknown type, type outside `attach_to`: the documented texts. A hidden anchor's text equals a missing one's.
- A caller without the read role: denial naming the rule, and no name lookup (a fake store fails if `Head` is called first).
- A poster without a post role: `channel_post_denied`, with no rate token charged.
- A signal type from stdio or CLI: `signals_not_kept` only after authz (an unauthorized caller gets the read or post denial instead).
- A hidden ref in props: `invalid_ref`, identical to a missing ref.
- Editing another author's message: `message_edit_denied`. An edit after the `within:` window: `edit_window_closed`.
- Removal without a reason where one is required: `reason_required`.
- Rate limit exceeded: the `rate_limited` text with a retry time.
- An unknown tool argument: refused by `addTool`.
- A watch with 17 targets, a 5th watch for one identity, a timeout of 51: refused with the existing errors.
- An invalid run-as value: exit 1. rela-server with the env var: refuses to start.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
1. Prompt injection via channel content. Mitigation: JSON-only delimiting, the leading notice, descriptions and instructions, CLI escaping, and a docs recipe to restrict agent writes with `client_baselines`. Residual: models may still follow injected text.
2. A watch leak or load from polling. Mitigation: gated replays on a fixed clock, a determinism test for `read_channel`, a poll only when channel targets exist and only for them, the existing caps, the per-identity replay slot, and a 5s remote poll. A raw head pre-check is a follow-up.
3. Process locality (CLI and stdio): signals refused; rate limits per invocation; posts reach the server SPA only on a refetch (T7) or phase-5 cross-process polling. Mitigation: documented, and decisions 5, 7 and 8 (follow-up ticket for server-side head polling).
4. Run-as inherits acl.yaml grants of the asserted name. Mitigation: local full trust; an `asserted_by` audit marker; docs.
5. Response size (16 KiB messages). Mitigation: MCP default page 20, max 50; paging with `before`.
6. Stale remote MCP dependencies after a Configure save (pre-existing pattern). Mitigation: documented, matching leases.
7. Widely used types change (`Principal` field, kong plimsoll). Mitigation: Equal, ShareKey and Identity tests; a justification comment.

Effort: m (two PRs: T6a MCP plus seams, T6b identity plus CLI).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: realtime config is documented in docs/realtime.md)
- [x] docs/cli-reference.md - New/changed commands (`rela channel ...`, global `--run-as` / `RELA_RUN_AS`)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: no SPA change; SPA is T7)
- [x] CLAUDE.md - New patterns or conventions (channel content to agents is JSON-delimited with `realtime.UntrustedNotice`; `--run-as` is local-only attribution)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] ~~N/A - Internal change~~ (N/A: user-facing docs are updated)

Also:
- `docs/mcp-server.md` (channel tools, watch loop, stdio limits)
- `docs/realtime.md` (process locality, agent recipe)
- `docs/acl-security.md` (run-as, self-asserted marker)
- docs-project guides sync

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-5XXIHY, RR-NZMJEY, RR-KEH5H1, RR-8Y12CY,
RR-LGYIVW, RR-ZKBD9P, RR-GAEVTB, RR-KS1OJC, RR-TEFECU, RR-G2UTG6, RR-TG9DAF,
RR-NTD0YJ, RR-II9BO2, RR-IZNXID, RR-ZAGUPL, RR-1IDCH3, RR-J4EHKV, RR-77QXF2,
RR-D5RUEV, RR-3E9XVD, RR-OFSS6Q, RR-TVTSRB, RR-NH1TDM, RR-7ORDGK, RR-LM249M,
RR-WOAZJU, RR-D0I0G1, RR-MXDRIX, RR-TP1MN8, RR-L9RKGE, RR-5UGBWS (all addressed
in the plan; decisions from Jeroen recorded in the plan's section 12)
