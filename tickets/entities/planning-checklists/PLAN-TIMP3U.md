---
id: PLAN-TIMP3U
type: planning-checklist
title: 'Planning: rela channel: run message actions and show session state'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: two `rela channel` subcommands over existing `*realtime.Service` methods.

- `rela channel action ID TYPE SEQ ACTION [--name N]` runs a message action (`Service.RunAction`), as `POST …/messages/{seq}/_action/{name}` does.
- `rela channel session ID TYPE [--name N]` shows the session (`Service.Session`), as MCP `read_session` does.

- `rela channel action ID TYPE SEQ --release [--name N]` releases a stuck action (`Service.ReleaseAction`), as `DELETE …/messages/{seq}/_action` does.

Out: an MCP tool for message actions; any change to the realtime service.

**Acceptance Criteria:**

1. `channel action` runs a declared promote: the message shows `action <name>: done -> <ref>`; `-o json` returns notice, message and warnings.
2. `channel action` refuses an undeclared action, a missing seq, and a caller whose role may not run it, with the service's user text and exit 1.
3. `channel session` prints the session line (state, waiting for, waiting on you) and the transitions the caller may post; `-o json` returns `realtime.SessionRead`.
4. `channel session` on a channel type without a session answers no_session; with realtime off, the off text.
5. `channel action --release` releases a pending action past the release delay; ACTION and `--release` together, or neither, is a usage error.
6. `channel action` counts as a write and `channel session` as a read for `--run-as` (identityWrites).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: effort s)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: thin CLI over an existing service)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `internal/cli/channel_mark_read.go`: command shape (`channelAddr`, `cs.svc()`, `channelFailed`, `writeChannelJSON`).
- `internal/cli/channel_render.go`: `sessionText`, and the message renderer already prints `action <name>: <status> -> <ref>`.
- `internal/dataentry/channels_handler.go` `runAction` and `internal/mcp/tools_channels.go` `handleSession`: the same service calls.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- New consumer-side interfaces in `channel.go`: `channelActor{RunAction, ReleaseAction}` and `channelSessionReader{Session}`, added to `channelService`. `*realtime.Service` already satisfies them; no wiring change.
- `ChannelActionCmd{channelAddr; Seq int64 arg; Action string optional arg; Release bool}`: seq must be 1 or more; exactly one of ACTION and `--release`. Calls `RunAction` or `ReleaseAction`; text output is `writeMessage(message, "action"/"released", nil, …)` followed by one `warning: <code> <path>: <detail>` line per warning, each part through `printable`; JSON is a tagged `{notice, message, warnings}` struct (`ActionResult` has no JSON tags).
- `ChannelSessionCmd{channelAddr}`: calls `Session`; text is the channel line, `sessionText`, and `may post: <transitions>`, all through `printable`; JSON is the `SessionRead` as is.
- Errors go through `channelFailed`, so the user text matches the other channel commands.
- identityWrites: `"channel action": true`, `"channel session": false`.

Alternative rejected: a `--action` flag on `channel post`. Running an action
acts on an existing message, not a new post, and HTTP models it as its own
route.

**Files to modify:** `internal/cli/channel.go`, new
`internal/cli/channel_action.go`, `internal/cli/identity.go`,
`internal/cli/channel_test.go` (or a new `channel_action_test.go`),
`docs-project/entities/guides/GUIDE-cli-reference.md` (then `just docs`).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** ID, type, name, seq and action name come from
argv. The CLI checks seq ≥ 1; the service validates the address, that the type
declares the action, and the gate.

**Security-Sensitive Operations:** RunAction creates an entity (promote). The
service applies acl.yaml to the ctx principal (stamped by the CLI identity,
including `--run-as`), the action's roles, the promoter's create check and the
read check on copied refs, exactly as for HTTP. The CLI adds no check of its own
and bypasses none. Errors go through `channelFailed`, which shows only
`realtime.UserText` and logs the rest.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Table tests in `internal/cli` against a real project
(`newChannelProject` with a realtime.yaml that declares a promote action and a
session), through `runChannel`: criteria 1-4 each a case, text and JSON.
Criterion 5 by the identityWrites completeness test that already exists.

**Edge Cases:** seq 0 or negative (refused before the service); action already
run (service conflict text); channel closed; named channel via `--name`.

**Negative Tests:** undeclared action, missing message, a role without the
action, realtime off, no session.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Low. The service owns every check; the risk is a CLI that skips the
identity stamp, which the shared kong path prevents, and a test with an
authorized and an unauthorized user pins it. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** `GUIDE-cli-reference.md` § rela channel (synopsis and
two short subsections), regenerated into `docs/cli-reference.md`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-0UW3B0 (significant, release added), RR-7RQD65
(minor, printable), RR-3OUAAH (minor, tagged JSON). All addressed.
