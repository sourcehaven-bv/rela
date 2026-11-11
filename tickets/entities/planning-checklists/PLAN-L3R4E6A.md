---
id: PLAN-L3R4E6A
type: planning-checklist
title: 'Planning: Hide message types from the web app composer'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- A new presentation key `channels.<type>.messages.<message>.offer: false`
in data-entry.yaml: people do not post the type in the web app. (Renamed from
the first draft's `offer: false` after the e2e run showed that a transition type
also gets an automatic decision button; agreed with the user on 2026-11-11.)
- The library (rela-components) honours it in `RlChannelComposer` (picker)
and `RlChannel` (whether Reply shows); the app drops the type's automatic
session decision button (`decisionViews`).
- Load-time checks: the key is known, the value is a real boolean, the
channel and message type exist; a warning when the type has a `button:` preset
in realtime.yaml.
- Docs in GUIDE-data-entry (channels presentation section).

Out of scope:
- Any change to `_actions.post`, the ACL, the HTTP API, MCP or the CLI. This
is presentation only. A person can still post the type through the API.
- Hiding messages of the type in the channel (`display` already does that).
- The inbox answer drawer: it answers a decision through its buttons, not
the free composer, so it is not affected.

**Acceptance Criteria:**
1. With `offer: false` on `pr-event`, the composer's type picker does not
offer `pr-event`, even when it is in `_actions.post`. A transition type with
`offer: false` gets no automatic decision button; a `button:` preset of it still
shows, with a load warning.
2. `pr-event` messages still render in the channel as usual.
3. Posting `pr-event` through the HTTP API (and so CLI/MCP) still succeeds.
4. A preset button, a compose button (`compose: true`) or the
"Other" answer that names a hidden type still works: the operator asked for that
type explicitly, so the composer opens with it.
5. Editing an existing message of a hidden type still opens the composer in
that type.
6. Reply shows only when a type the picker offers can reply.
7. When every postable type is hidden, the composer renders nothing (as with
nothing postable today).
8. An unknown key or a non-boolean value under the message presentation is
refused at load.
9. `offer: false` on a type with a `button:` preset warns at load.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small presentation key)
- [x] ~~Searched for existing libraries~~ (N/A: in-app config key)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: follows the existing `display` key)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `MessagePresentation.Display` (TKT-03DMXH,
internal/dataentryconfig/channels.go) is the same kind of key: web-app
presentation only, data stays reachable through API/MCP/CLI.
- `buttonTypes` in RlChannelComposer already removes types from the picker,
with a fallback to all offered types when that would leave none. The new key is
a stricter filter without that fallback.
- Workaround: a separate agent role in acl.yaml (heavy; changes access).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- Go: add `Offer *bool` (`yaml/json:"offer,omitempty"`) to
`dataentryconfig.MessagePresentation`; add `offer` to `messagePresentationKeys`.
YAML decoding refuses a non-boolean value. The config travels to the SPA through
the existing `channels` block.
- Frontend types: `offer?: boolean` on the app config type and on the
library's `ChannelMessagePresentation`; `presentationView` passes it on.
- RlChannelComposer: `offered` stays as is (so edits and compose decisions
keep working). `pickable` drops types whose presentation says `offer: false`,
before the button-types filter; the button fallback never brings a hidden type
back. A compose decision of a hidden type stays selectable through
`liveCompose()`. With nothing pickable and no live compose decision the composer
renders nothing.
- RlChannel `canReply` applies the same filter.
- `decisionViews` takes an `unoffered` predicate (from the presentation via
`useSessionDecisions`) and skips the derived type button; presets stay.
- `MessageFacts.HasPresetButtons` (appbuild adapter over the compiled
session) feeds the load warning.

Alternatives rejected:
- Key in realtime.yaml: realtime.yaml is structural; this is presentation.
data-entry.yaml's `channels:` is where `display` lives.
- Filter `_actions.post` server-side for web clients: makes the API differ by
client and is access-shaped; presentation belongs in the SPA.
- Filtering `postable` in OpenChannel.vue: would also break compose
decisions and edits of hidden types.

**Files to modify:**
- internal/dataentryconfig/channels.go (+ test)
- frontend/src/types/config.ts
- frontend/src/components/entity/channels/channelView.ts (+ test)
- frontend/src/components/entity/channels/useSessionDecisions.ts, OpenChannel.vue
- internal/appbuild/realtime_channels.go (+ test)
- e2e/tests/channels.spec.ts, e2e/tests/realtime-project.ts
- frontend/packages/rela-components/src/components/channel/types.ts
- frontend/packages/rela-components/src/components/channel/RlChannelComposer.vue (+ test)
- frontend/packages/rela-components/src/components/channel/RlChannel.vue (+ test)
- docs-project/entities/guides/GUIDE-data-entry.md (and generated docs/data-entry.md)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- data-entry.yaml (operator config): known-key allowlist, boolean decode.
Invalid input fails the load with a path-qualified message.

**Security-Sensitive Operations:**
- None. The key is not access control and the docs say so; ACL stays the
only gate on posting.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1, AC7: RlChannelComposer unit tests (picker options; all hidden →
renders nothing).
- AC4: compose decision of a hidden type selects it; AC5: edit of a hidden
type opens in that type.
- AC6: RlChannel test: Reply hidden when only hidden types reply.
- AC2: RlChannel renders a message of a hidden type.
- AC3: no server change; existing post tests cover it. Manual check through
`just dev` with the API.
- AC8: dataentryconfig load tests (unknown key, `offer: "no"`).
- channelView test: `presentationView` carries `composer`.
- Manual: `just dev` on a project with a hidden type, check the picker in
the browser.

**Edge Cases:**
- `offer: true` is the default and changes nothing.
- Hidden type is also a button type: stays out of the picker; the button
still shows.
- Every non-button type hidden: the button fallback must not bring hidden
types back.

**Negative Tests:**
- Non-boolean `composer` value fails load; a typo such as `compose:` fails as
an unknown key.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Operators read it as access control. Mitigation: docs state it is
presentation only, next to the acl.yaml route for real restriction.
- Composer selection logic is intricate (compose decisions, edits). Mitigated
by keeping `offered` unchanged and filtering only the picker.

Effort: s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md (via GUIDE-data-entry) - channels presentation

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small presentation-only key following the existing `display` pattern; plan reviewed with the user)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
