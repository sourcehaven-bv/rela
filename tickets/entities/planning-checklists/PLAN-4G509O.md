---
id: PLAN-4G509O
type: planning-checklist
title: 'Planning: Built-in typing indicator per channel type'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** Today a typing indicator is an operator-declared ephemeral message
type, for example `typing: { persist: ephemeral, ttl: 5s, rate: 1/s }` in
`realtime.yaml` (see the docs example in `GUIDE-realtime.md:55` and
`internal/realtimecfg/testdata/valid.yaml:67`). The composer lists it as a tab
next to the real message types, and the user must post it by hand. This ticket
makes typing a built-in signal. A channel type switches it on with `typing:
true`, and it then works without further setup.

Terms used below:
- **Typing entry**: the built-in signal. It means "this person is writing a
log message in this channel now". It has an author, a start time (`since`) and a
server-side deadline. It has no properties.
- **Operator signal**: a message of an operator-declared ephemeral message
type. These keep working unchanged, in the existing ring.
- **Ring**: the in-memory list of live operator signals of one channel
(`internal/realtime/ephemeral.go`, `ephemeralStore`).
- **Typing store**: a new, separate in-memory map per channel, keyed by
identity, that holds typing entries.
- **Identity**: the rate-limit identity, user plus organization
(`bucketIdentity`, `internal/realtime/ratelimit.go:143`). The tool is not part
of it, so a user's browser tabs and agent tokens are one identity.
- **Log message type**: a message type with `persist: log` (the default).
- **Mark**: `Local.mark(key)`, which wakes live subscribers (the SPA's live
hub and MCP watches) for a channel.

**Scope:**
- In: the channel type key `channel_types.<name>.typing`, a boolean, default
`false`. It loads into `realtimecfg.ChannelType.Typing`.
- In: load checks. The value must be a boolean as yaml.v3 decodes one (so
`yes`/`no`/`on`/`off` stay accepted, as for every existing boolean key). A type
with `typing: true` needs at least one log message type. With `typing: true`,
the message type name `typing` is reserved.
- In: the key checker names the key path for a boolean key whose value
yaml.v3 would refuse to decode. The accepted set does not change for any key;
only the message for an already-refused value gains its key path.
- In: constants in `realtimelimits`: `TypingTTL` (6s), `TypingRate` (1/s per
identity and channel), `MaxTypingPerChannel` (32 entries) and
`MaxTypingChannels` (4096 channels with entries).
- In: `realtime.Service.Typing`, which starts or extends the caller's typing
entry in the channel. It is process-local, never stored, never audited, and
creates no channel.
- In: wake model. Readers are marked only when an entry starts (the identity
had no live entry in that channel), when a log post clears it, and when it
expires on the server. A refresh only moves the server-side deadline and marks
nothing. The server is the clock; clients do no expiry of their own.
- In: readers receive the live entries in the head as `typing_now:
[{author, since}]`, without their own identity's entry, only while the channel
is open and the type has typing on. MCP `read_channel` shows `typing_now` when
signals are requested. Both use one shared helper.
- In: `typing` on the channel type in `GET /api/v1/_schema/channels`, and
`_actions.typing` (the caller may send typing here now) in the HTTP head and in
MCP `read_channel` actions.
- In: HTTP route `POST …/_channels/{type}[/{name}]/_typing` (202 with
`ttl_seconds`). OpenAPI path and schemas.
- In: SPA. The composer emits `typing` while the user types a log message
type; `OpenChannel` sends it throttled to half the TTL and resets the throttle
after a send. `RlChannel` shows "<name> is typing…" exactly as the last head
says, in its own atomic live region.
- In: docs in `docs-project/entities/guides/GUIDE-realtime.md` (then
`just docs`), and the `read_channel` note in `GUIDE-mcp-server.md`.
- Out: an MCP tool to send typing. An agent cannot refresh during a
generation that outlasts the 6s TTL, so the value is low; it can be added later
behind its own capability.
- Out: `typing` on the channel view and in the CLI. The schema endpoint and
`_actions.typing` carry it.
- Out: an explicit "stopped typing" call (a DELETE). The entry ends by TTL or
on send.
- Out: a grouping key for built-ins. Future built-ins are sibling keys.
- Out: cross-process delivery. Typing is process-local, as operator signals
are.
- Out: migrating existing configs. An operator `typing` ephemeral type keeps
working while `typing` is off.

**Acceptance Criteria:**
1. `typing: true` loads and sets `ChannelType.Typing`; absent means false.
Test `TestParse_Typing` in `internal/realtimecfg/load_test.go`: a type with
`typing: true` and a log type, a type without the key, a type with `typing:
false`, and a type with `typing: yes` (loads as true, yaml.v3 rule).
2. An invalid value is refused with its key path. Table-driven
`TestParse_TypingInvalid`: `typing: 1` and `typing: maybe` give
`channel_types.<t>.typing: must be true or false`; `typing: { on: true }` gives
"must be a single value"; `typing: true` on a type whose message types are all
ephemeral gives `channel_types.<t>.typing: typing announces a message to come;
declare a log message type or drop typing`; `typing: true` with a message type
named `typing` gives `channel_types.<t>.message_types.typing: "typing" is
reserved in a channel type with typing: true: the built-in indicator replaces
it; remove this message type`. A top-level `typing:` stays a plain unknown key.
`TestParse_BoolKeysUnchanged`: `final: yes`, `required: on` and
`reason_required: off` still load with the same values as before; a `creator: 1`
row is still refused, now with its key path.
3. A channel type that declares an operator `typing` ephemeral type and no
`typing:` key loads and posts it as before. Existing `TestEphemeral_*` tests
pass unchanged, and `valid.yaml` keeps its `incident-room.typing` operator type.
4. Turning typing on or off passes `CheckReload`. Test
`TestCheckReload_TypingToggleAllowed`.
5. The composer never lists typing: the schema endpoint's `message_types`
and the head's `_actions.post` never contain a built-in typing entry. Tests
`TestChannelSchema_Typing` (`channels_schema_test.go`: `typing: true` on the
type, no `typing` message type) and story `RlChannelComposer.stories.ts`
`TypingIsNotAMessageType`.
6. Sending typing keeps an entry and stores nothing. Service test
`TestTyping_KeptNotStored` (`internal/realtime/typing_test.go`): after `Typing`,
another reader's `Head` lists the sender in `TypingNow` with `Since = now`;
`store.Head` on a cardinality-one channel with no message still answers
`channels.ErrNotFound`; the audit and message log are empty; the subscription is
woken once for the key.
7. A second reader sees it, the sender does not. `TestTyping_SelfExcluded`:
reader B sees A; A's own head (same user, other tool) does not list A.
8. A refresh is silent. `TestTyping_RefreshSilent`: A types three times
within the TTL (advancing the clock 1s each); the subscription is woken once (at
start); `Since` stays at the first time; the head JSON is byte-identical after
each refresh; the deadline moved (the entry is still live 6s after the last
refresh, gone after).
9. The entry ends after the TTL. `TestTyping_Expires`: advance the test
clock past the deadline; the expiry timer drops it and marks the key once; the
head no longer lists it.
10. The entry ends on send. `TestTyping_ClearedOnPost`: A types, A posts a
log message; B's head no longer lists A; the post marked the key once. An
operator signal post by A does not clear it (`TestTyping_NotClearedBySignal`).
11. Typing and operator signals never evict each other.
`TestTyping_SeparateFromRing`: A at its ring share and at
`MaxEphemeralChannelsPerIdentity` rings still starts typing in a fifth channel;
A's operator signals are all still live; a later operator post by A leaves the
typing entry in place.
12. Typing caps refuse rather than evict. `TestTyping_Caps`: the 33rd
identity in one channel gets 429; with `MaxTypingChannels` reached, a start in a
new channel gets 429 while a refresh in a held channel passes; one identity has
at most one entry per channel.
13. Refusals, table-driven `TestTyping_Refused`, each with the type and
status: typing off (403 `typing_not_offered`); no read role (403
`channel_read_denied`, before anything else); reader with no post role of any
log type (403 `channel_post_denied`, naming the roles); no identity (403
`no_principal`); closed and archived channel (409 `channel_not_open`); over
`TypingRate` (429 `rate_limited` with Retry-After); unknown many-type channel
name (404 `channel_not_found`). No refusal creates a channel, an entry, or
charges a token before the role check.
14. Typing has its own rate bucket and `signal_rate` does not apply:
`TestTyping_OwnBucket` with `limits.signal_rate: 1/m` sends typing at 1/s; an
operator signal in the same second still passes its own bucket.
15. The head of a closed channel, or of a type whose typing a reload turned
off, lists no typing entry although the store still holds one.
`TestTyping_HiddenWhenClosedOrOff`.
16. `_actions.typing` matches the write for the cases the head can express:
typing off, no post role of any log type, no principal, closed, archived, and
the allowed case. `TestTyping_AffordanceMatches` runs these through `Head` and
`Typing`. The 429 rate limit is outside the affordance contract, as it is for
`_actions.post`.
17. HTTP: `POST …/_typing` answers 202 `{ttl_seconds}`; GET is 405 with
`Allow: POST`; a non-empty body other than `{}` or an oversized body is refused
like other channel bodies; realtime off is the uniform 404; an anchor the caller
cannot read, or that does not exist, answers the same uniform 404 as `GET` head,
charges no token and creates no entry. Handler tests `TestChannelTyping_*` in
`channels_handler_test.go`; golden wire update in `channels_wire_golden_test.go`
for `_actions.typing` and `typing_now`; `parseChannelRoute` rows for `_typing`
on one and many types.
18. Live frames: `TestLive_TypingFrames` in
`internal/dataentry/live_channels_test.go`: a subscribed reader gets one frame
at start, none for two refreshes within the TTL, and one at expiry.
`TestLive_TypingReloadRound`: a reload that turns typing off runs the full round
(`liveHub.policyChanged`) and the reader's next head has an empty `typing_now`.
19. OpenAPI: the `_typing` path exists only for types with typing on; the
`ChannelActions`, `ChannelHead` and channel type descriptor schemas carry the
new fields; `TypingAccepted` and `TypingEntry` schemas exist. Test rows in
`internal/openapi/channels_test.go`.
20. MCP: `read_channel` shows `actions.typing` and, with signals,
`typing_now` without the caller's own entry, and `signals` holds no typing entry
(`TestReadChannel_Typing`). A watch on `read_channel` with signals returns once
at start and not on refreshes (`TestReadChannel_TypingRefreshDoesNotWakeWatch`
in `tools_watch_channels_test.go`).
21. SPA: typing in the composer sends at most one request per half TTL, none
while editing, none while an ephemeral operator type is selected, none when
`_actions.typing` is false; a 429 or other error backs off without a toast;
after a post the next keystroke sends again. Unit tests in
`src/composables/useTyping.test.ts`. Composer story `EmitsTyping` (play: emits
for a log type, not for an ephemeral type, not while editing). Indicator: story
`RlChannel.stories.ts` `Typing` with rows for one, two, three and five typists
("Ann is typing…", "Ann and Bo are typing…", "Ann, Bo and 1 other are typing…",
"Ann, Bo and 3 others are typing…"), and a play step that a head with the same
typists leaves the text node unchanged. Wire mirror: `CHANNEL_WIRE_FIELDS`
updated and `wireFields.test.ts` passes.
22. E2E: two browser contexts on one channel with `typing: true`; B types,
A sees "B is typing…"; B sends, the indicator disappears. Test in
`e2e/tests/channels.spec.ts`.
23. Docs: `GUIDE-realtime.md` describes `typing: true`, its checks, the
route, `typing_now`, the wake model and the SPA behaviour; the example uses
`typing: true` instead of the operator type; `just docs` regenerates
`docs/realtime.md`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: covered by RES-1UCUBA realtime research)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-1UCUBA

**Existing Solutions:**
- Libraries: none needed. The work reuses the expiry timer, rate limiter and
live-change marks; a pub/sub library would add a second signal path.
- Ring and caps (pattern for the typing store, not reused for storage):
`internal/realtime/ephemeral.go:88` (`add`, refuse-not-evict caps), `:178`
(`live`), `:194` (sweep returning changed keys), `:290` (`dropAnchor`).
- Expiry timer and marks: `internal/realtime/local.go:177-219` (`expire`,
`scheduleExpiry`, `fire`), `:223-250` (anchor drop on delete and rename).
- Signal post path: `internal/realtime/service.go:300-354` (`Post`: order of
checks, open check, rate) and `:422-442` (`postSignal`).
- Head assembly: `internal/realtime/service.go:163-195`; affordances
`:197-237`. Read path for MCP: `internal/realtime/read.go:103-152`.
- Implicit cardinality-one channel: `internal/realtime/authz.go:216-244`
(`loadHead`: a missing one-channel reads as an empty open channel). A signal
never calls `record` or `store.Append`.
- Rate limits: `internal/realtime/ratelimit.go:151` (`take`: per (identity,
channel, op) bucket plus the identity aggregate `IdentityRate`).
- Reserved-name precedent: `internal/realtimecfg/session.go:17-20,152-154`.
- Key-path machinery: `internal/realtimecfg/keys.go:52-120`.
- Built-in caps: `internal/realtimelimits/limits.go:15-60`.
- Config-declared capability refusal precedent: `TypeLifecycleNotOffered`
(403) in `internal/realtime/errors.go:52`.
- HTTP routes and reserved segments: `internal/dataentry/channels_handler.go:22-25`,
`:236-272` (`parseChannelRoute`), `:366-382` (post, 202 for a signal). Channel
names match `^[0-9a-z]{1,32}$` (`internal/channels/channels.go:164`), so
`_typing` can never be a channel name.
- Live hub: `internal/dataentry/live.go:45-49` (replay slots and round cap),
`:499-511` (`policyChanged`), `live_topics.go:51-56` (head and list topics per
mark). This is why marks must stay rare.
- Wire: `internal/apiwire/v1/channels.go:100-105,203-216,265-275`; mapping
`internal/dataentry/channels_wire.go:117-146`, `channels_schema.go:33-51`;
describe view `internal/realtime/describe.go:20-34`.
- OpenAPI: `internal/openapi/channels.go:152` (per-type paths), `:467`.
- MCP read and watch: `internal/mcp/tools_channels.go:346-363`,
`tools_watch.go:380-395`.
- SPA: `frontend/src/composables/useChannel.ts:285-312` (a frame on a type
with signals refetches the head), `OpenChannel.vue:92-125`,
`frontend/src/types/channels.ts:317` (`CHANNEL_WIRE_FIELDS`).
- Library: no typing indicator component exists. `RlChannel.vue:324-329`
draws operator signals in a polite live region; `RlChannelComposer.vue:96` emits
post/save; text in `composables/useMessages.ts:74,288`.
- Reference implementations: Matrix (`m.typing`: client re-sends before a
server timeout, cleared on send, server decides expiry), Slack and Mattermost
(client throttle, server TTL, never persisted). All key typing per user, not per
device, and hide the typist's own indicator.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Decisions:
- **D1 Wire shape: a dedicated route**, `POST …/_channels/{type}[/{name}]/_typing`
with an empty body or `{}`. It cannot clash with an operator message type, needs
no props validation, and keeps typing out of `message_types` and
`_actions.post`.
- **D2 Name clash.** With `typing: true`, a message type named `typing` is
refused at load (as `session` is in a session type). With typing off, an
operator `typing` type works as today.
- **D3 Readers receive it in the head** as `typing_now: [{author, since}]`,
not inside `ephemeral`. Nothing in an entry moves on refresh, so the head and
MCP digests change only on start and stop.
- **D4 Wake model.** `Typing` marks the channel only when it starts an
entry. A log post that clears an entry marks (the post marks already). The
expiry timer marks when the server drops an entry. A refresh moves the deadline
and marks nothing. Clients never expire typing themselves; they show what the
last head says. This removes clock-skew effects and keeps live-hub replays and
MCP watch wakes to two per typing burst.
- **D5 Who sees it:** anyone who passes the channel's read check. The list
leaves out the caller's own identity, is empty unless the channel is open and
the type has typing on, and is sorted by `since`, then author user.
- **D6 Who may send:** a caller who holds a post role of at least one log
message type, in an open channel. No separate role. MCP has no send tool, so
only the HTTP route sends; there is no LogOnly path.
- **D7 Storage: a separate typing store** in `Local`: per channel key a map
identity → entry `{author, since, deadline}`. One entry per identity per channel
by construction. Caps refuse with 429 and never evict: `MaxTypingPerChannel`
entries in one channel, `MaxTypingChannels` channels with entries in the
process. It never touches the ring, so typing and operator signals cannot evict
each other or take each other's slots.
- **D8 Implicit channels:** `Typing` never calls `record` or a store write.
A cardinality-one channel with no message stays unstored. A many-type channel
must exist (`loadHead` 404).
- **D9 Rate:** its own bucket, op `typing`, at `TypingRate` (1/s, burst 1)
per identity and channel, plus the identity aggregate (`IdentityRate`) as for
every write. `limits.signal_rate` does not apply.
- **D10 Closed channel:** sending is 409 `channel_not_open`. Live entries
are hidden from the head at once (D5) and expire within the TTL.
- **D11 Throttle source:** the 202 answer carries `ttl_seconds`; the SPA
refreshes at `ttl_seconds / 2` (3s), so one user in two tabs stays under 1/s and
no constant is copied into TypeScript.
- **D12 Bool check:** the key checker decodes a boolean field's scalar with
yaml.v3 itself (`node.Decode(&b)`) and reports a failure with the key path. The
accepted set is exactly what loads today.

Steps:

1. `internal/realtimelimits/limits.go`: add `TypingTTL = 6 * time.Second`,
`MaxTypingPerChannel = 32`, `MaxTypingChannels = 4096` to the const block and
`TypingRate = Rate{Count: 1, Per: time.Second}` to the rate vars, with godoc
saying the server owns them and `limits:` cannot change them. `limits_test.go`:
`TypingTTL <= MaxEphemeralTTL`, `TypingRate.AtMost(DefaultEphemeralRate)`.
2. `internal/realtimecfg/raw.go`: `Typing bool \`yaml:"typing"\``on`rawChannelType`.
3. `internal/realtimecfg/keys.go` `walk`, default branch: when
`t.Kind() == reflect.Bool` and `n.Kind == yaml.ScalarNode`, run `var b bool; if
n.Decode(&b) != nil { add(path, "must be true or false") }`.
4. `internal/realtimecfg/compiled.go`: `Typing bool` on `ChannelType` with
godoc. New file `typing.go`: `const TypingName = "typing"` and `func (v
*validator) typing(key string, ct *ChannelType)`.
5. `internal/realtimecfg/validate.go` `channelType`: after message types,
`if raw.Typing { ct.Typing = true; v.typing(key, ct) }`, adding the two problems
of AC 2. Shape is unchanged, so `CheckReload` allows a toggle.
6. New `internal/realtime/typingstore.go`:
   - `type typingEntry struct { author channels.Author; since, deadline
time.Time }`.
   - `type typingStore struct { mu sync.Mutex; perChannel, channels int;
byKey map[channels.Key]map[string]typingEntry }` with
`newTypingStore(perChannel, channels int)`.
   - `touch(key, identity, author, now) (started bool, expired
[]channels.Key, err error)`: sweeps expired first; an existing live entry gets
`deadline = now+TypingTTL` (started false); a new entry checks both caps (429
via `rateLimited`, retry at the soonest deadline that frees room), then stores
`since = now` (started true).
   - `clear(key, identity) bool`, `live(key, now) []typingEntryWithIdentity`
(sorted by `since`, then author user), `sweep(now) []channels.Key`,
`nextDeadline() (time.Time, bool)`, `dropAnchor(id, face) []channels.Key`.
7. `internal/realtime/local.go`: `Local` gains `typing *typingStore`
(constructed in `newLocal`). `expire` marks the keys of both sweeps.
`scheduleExpiry` arms for the earlier of `ephemeral.nextExpiry` and
`typing.nextDeadline`. `dropAnchor` and `EntityRenamed` drop typing entries too.
8. New `internal/realtime/typing.go`:
   - `type TypingResult struct { TTL time.Duration }`.
   - `type TypingView struct { Author AuthorView \`json:"author"\`; Since
time.Time \`json:"since"\` }`.
   - `func (s *Service) Typing(ctx, addr RawAddress) (TypingResult, error)`.
Order: `open` (read check, uniform 404 for an unreadable anchor),
`requireCaller`, `!c.ct.Typing` gives 403 `typing_not_offered`, `!c.mayType()`
gives `typingDenied(c.ct)` (403 `channel_post_denied`, rule
`channel_types.<t>.typing`, roles = union of the log types' `post`),
`requireOpen`, `limiter.take(..., opTyping, TypingRate)`, then `typing.touch`.
Mark the expired keys; mark `c.key` and call `scheduleExpiry` only when
`started`.
   - `func (c *call) mayType() bool` (any log type with `c.mayPost`),
`func (c *call) typingAllowed() bool` (`ct.Typing`, caller present, state open,
`mayType`).
   - `func (s *Service) liveSignals(c *call) (operator []signal, typing
[]TypingView)`: the one helper Head and Read use. `operator` is the ring's live
signals; `typing` is the typing store's live entries for `c.key` without
`bucketIdentity(c.p)`, empty unless `ct.Typing` and the channel is open.
   - `opTyping = "typing"` in `ratelimit.go`; `TypeTypingNotOffered =
"typing_not_offered"` in `errors.go`.
9. `internal/realtime/service.go`:
   - `HeadView` gains `TypingNow []TypingView \`json:"typing_now"\``(always non-nil);`Head` uses the helper.
   - `Affordances` gains `Typing bool \`json:"typing"\``, set in
`affordancesWith`from`c.typingAllowed()`.
   - `Post`: after a successful `store.Append`, call
`s.local.typing.clear(c.key, bucketIdentity(c.p))`; the existing
`s.local.mark(c.key)` covers the wake.
10. `internal/realtime/read.go`: `ReadView` gains `TypingNow []TypingView
\`json:"typing_now,omitempty"\``, filled through the helper when `req.Signals`;
`Signals` holds operator signals only.
11. `internal/realtime/describe.go`: `ChannelTypeView.Typing bool`.
12. `internal/apiwire/v1/channels.go`: `ChannelActions.Typing bool
\`json:"typing"\``, `ChannelHead.TypingNow []TypingEntry \`json:"typing_now"\``,
`type TypingEntry struct{Author ChannelAuthor \`json:"author"\`; Since string
\`json:"since"\`}`, `type TypingAccepted struct{TTLSeconds int
\`json:"ttl_seconds"\`}`, `ChannelTypeSchema.Typing bool \`json:"typing"\``.
13. `internal/dataentry/channels_handler.go`: `typingSegment = "_typing"`;
`parseChannelRoute` keeps it out of the name slot and accepts `len(tail)==1 &&
tail[0]==typingSegment` as `rt.typing`; `serve` routes it with
`allowMethod(POST)`. New consumer interface `channelTypist { Typing(ctx, addr
realtime.RawAddress) (realtime.TypingResult, error) }` and field `typist` set in
`SetChannels`. Handler `typing` reads the body with `decodeChannelBody(...,
emptyOK=true)` and answers 202 `TypingAccepted`. Update the route table comment.
`channels_wire.go` and `channels_schema.go` map the new fields;
`channels_errors.go` gets the status for `typing_not_offered` if it lists types.
14. `internal/openapi/channels.go`: when `ct.Typing`, add `POST
{base}/_typing` (202 `TypingAccepted`; 403, 404, 409, 429). Add the fields and
the `TypingAccepted` and `TypingEntry` schemas.
15. `internal/mcp/tools_channels.go`: no new tool or interface. The
`read_channel` description mentions `actions.typing` and `typing_now`, and that
`typing_now` is empty under stdio (process-local).
16. Library (`frontend/packages/rela-components`):
    - `types.ts`: `ChannelTypingView { key: string; author:
ChannelAuthorView }`.
    - `useMessages.ts`: `channelTyping: (args: { names: string[]; others:
number }) => string`. The English default handles one and two names and "1
other" / "N others".
    - `RlChannelComposer.vue`: emit `typing` on input while not editing,
the selected message type has `persist: 'log'`, and the form holds a non-empty
value.
    - `RlChannel.vue`: prop `typing?: ChannelTypingView[]`; the component
passes at most two names plus `others`; the text renders in its own element with
`aria-live="polite"` and `aria-atomic="true"`, apart from the operator signal
region.
    - Stories with play functions: `RlChannel.stories.ts` `Typing`,
`RlChannelComposer.stories.ts` `EmitsTyping` and `TypingIsNotAMessageType`.
17. App (`frontend/src`):
    - `types/channels.ts`: new fields and types; `CHANNEL_WIRE_FIELDS`
entries for `ChannelActions`, `ChannelHead`, `TypingEntry`, `TypingAccepted`,
`ChannelTypeSchema`.
    - `api/channels.ts`: `sendTyping(t: ChannelTarget):
Promise<TypingAccepted>`.
    - New `composables/useTyping.ts`: `useTyping(target, allowed)` returns
`{ onInput(), reset() }`. `onInput` sends when `allowed()` and `Date.now() >=
nextAt` (a local throttle only; no server time is compared); on success `nextAt
= now + ttl_seconds*500`; on any error `nextAt = now + 3000` and no toast.
`reset()` sets `nextAt = 0`. A target change resets.
    - `OpenChannel.vue`: `useChannel(..., { ephemeral: hasEphemeral ||
channelType.typing })`; composer `@typing="typing.onInput"`; after a successful
post `typing.reset()`; pass `typingViews(head.typing_now)` to `RlChannel`.
`useExpiryClock` is not applied to `typing_now`.
    - `channelView.ts`: `typingViews(entries)`.
18. Config fixtures: `internal/realtimecfg/testdata/valid.yaml` gains a
`typing: true` channel type (keep the operator `typing` type on
`incident-room`); helpers in `internal/realtime/helpers_test.go`,
`internal/dataentry/channels_env_test.go`,
`internal/mcp/channels_fixture_test.go` gain a typing type;
`e2e/tests/realtime-project.ts` sets `typing: true`.
19. Docs (see Documentation Planning).

**Alternatives considered:**
- Reserved message type name on `POST …/messages`: clashes with existing
operator types named `typing` and needs special cases in validation,
`_actions.post` and `message_types`. Rejected (D1).
- Typing inside `ephemeral` with a reserved `type`: every client would draw
it as a generic signal line. Rejected (D3).
- Typing in the existing ring: it would share ring slots, ring-share and
rings-per-identity caps with operator signals, so one could evict or block the
other. Rejected for a separate store (D7).
- Marking on every refresh with a moving `expires_at`, and client-side
expiry: each refresh replays the head and list topics for every reader, triggers
two client requests per reader, wakes MCP watches every 3s, and flickers under
clock skew. Rejected for the start/stop wake model (D4).
- A general bool check that refuses `yes`/`on`: breaks configs that load
today. Rejected (D12).
- An MCP `send_typing` tool: the TTL is shorter than a typical generation.
Deferred.
- `typing` on every channel view: duplicates a channel type property already
in the schema endpoint and `_actions.typing`. Rejected.
- Applying `limits.signal_rate`, a separate typing role, per-tab identity,
and creating a channel on first typing: rejected for the reasons in D6, D8, D9
and the agreed design.

**Dependencies:** no new modules. Go packages touched: `realtimelimits`,
`realtimecfg`, `realtime`, `apiwire/v1`, `dataentry`, `openapi`, `mcp`
(description and tests only). Frontend: `rela-components` (channel components,
`useMessages`), app `api/channels.ts`, `types/channels.ts`, `OpenChannel.vue`,
new `useTyping.ts`. No new package imports.

**Files to modify:**
- `internal/realtimelimits/limits.go`, `limits_test.go`
- `internal/realtimecfg/raw.go`, `keys.go`, `compiled.go`, `validate.go`,
new `typing.go`, `load_test.go`, `testdata/valid.yaml`
- `internal/realtime/local.go`, `service.go`, `read.go`, `describe.go`,
`errors.go`, `ratelimit.go`, new `typing.go`, new `typingstore.go`, new
`typing_test.go`, `helpers_test.go`
- `internal/apiwire/v1/channels.go`
- `internal/dataentry/channels_handler.go`, `channels_wire.go`,
`channels_schema.go`, `channels_errors.go`, `channels_handler_test.go`,
`channels_wire_golden_test.go` (+ golden files), `channels_schema_test.go`,
`channels_env_test.go`, `live_channels_test.go`
- `internal/openapi/channels.go`, `channels_test.go`
- `internal/mcp/tools_channels.go` (description), `channels_fixture_test.go`,
`tools_watch_channels_test.go`, read_channel tests
- `frontend/packages/rela-components/src/components/channel/types.ts`,
`RlChannel.vue`, `RlChannelComposer.vue`, `RlChannel.stories.ts`,
`RlChannelComposer.stories.ts`, `src/composables/useMessages.ts`
- `frontend/src/types/channels.ts`, `src/api/channels.ts`, new
`src/composables/useTyping.ts` + test,
`src/components/entity/channels/OpenChannel.vue`, `channelView.ts`,
`channelView.test.ts`, `wireFields.test.ts`
- `e2e/tests/realtime-project.ts`, `e2e/tests/channels.spec.ts`
- `docs-project/entities/guides/GUIDE-realtime.md`,
`docs-project/entities/guides/GUIDE-mcp-server.md`, generated docs via `just
docs`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- `realtime.yaml` `typing:` (operator config): accepted only if yaml.v3
decodes it as a boolean; the validator adds the log-type and reserved-name
rules. Config is not secret, so messages name keys and values.
- HTTP route segments: `{type}` resolves against the config; `{name}` must
match the channel name pattern; `_typing` is matched exactly. The body is empty
or `{}`; anything else is refused by `decodeChannelBody` with the existing size
limit. No user text reaches the typing store.
- Author identity comes from the authenticated principal, never from the
request.

**Security-Sensitive Operations:**
- Authorization: `open` runs the read check first, so an unreadable or
missing anchor answers the uniform 404 before any token, entry or channel exists
(entity existence is the secret). The write gate is the post roles of the log
types (D6). `_actions.typing` follows the contract for the cases in AC 16.
- Read-side disclosure: `typing_now` names authors to readers only, the
same audience that sees messages and operator signals. Self-exclusion is a UX
choice, not a security property.
- Resource exhaustion: one entry per identity per channel,
`MaxTypingPerChannel` and `MaxTypingChannels` caps (refuse, never evict), the
per (identity, channel) `TypingRate` bucket, the identity aggregate
`IdentityRate` and the limiter's bucket caps. Refreshes do not mark, so a typist
cannot drive live-hub replays faster than start and stop. No store write, no
audit growth.
- No file access, no crypto, no external I/O, no Lua on this path.
- Errors: refusals reuse `DenialError` with fixed titles; config rules and
role names appear (config is not secret); no entity content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** one named test or table row per acceptance criterion, as
listed there. Layers: `realtimecfg` unit tests (AC 1 to 4); `realtime` service
tests over the file store on MemFS with the test clock (AC 6 to 16); HTTP
handler, golden wire and live-hub tests (AC 17, 18); OpenAPI (AC 19); MCP read
and watch (AC 20); frontend unit tests and story play tests (AC 5, 21);
Playwright e2e across two browser contexts (AC 22); docs (AC 23). Race detector
on. `just arch-lint`, `just plimsoll` (Service stays under 40 methods), `just
comment-lint`.

**Edge Cases (each has a test):**
- Cardinality-one channel with no message: typing keeps it unstored (AC 6).
- Many-type channel name that does not exist: 404 (AC 13).
- Hidden or missing anchor: uniform 404, nothing charged or created (AC 17).
- Same user in two tabs or with an agent token: one entry, not shown to
self (AC 7).
- Refresh within TTL: no mark, `since` kept, deadline moved (AC 8, 18, 20).
- Typing then log post: cleared; typing then operator signal: kept (AC 10).
- Operator signals at every ring cap: typing still starts; typing never
evicts them (AC 11).
- Channel closed while someone types: hidden at once, expires later (AC 15).
- Reload turns typing off while entries are live: full round, hidden (AC
15, 18).
- `limits.signal_rate` lowered: typing unaffected (AC 14).
- Operator type named `typing` with typing off: unchanged (AC 3).
- Entity deleted or renamed: typing entries dropped and marked (row in
`TestTyping_Caps` or a new `TestTyping_DropOnDeleteAndRename`).
- SPA: editing, an ephemeral type selected, `_actions.typing` false: no
send; 429 silent with backoff (AC 21).
- Same typists in a new head: indicator text unchanged, not re-announced
(AC 21).

**Negative Tests:** AC 2 (every load rule; bool keys still accept the yaml.v3
words), AC 12 (caps), AC 13 (every refusal and its status), AC 17 (wrong method,
bad body, realtime off, hidden anchor).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Live-hub load: each typing burst causes two marks (start and stop), each
replaying the head and list topics for every subscribed reader. Mitigation:
refreshes are silent (D4); `TestLive_TypingFrames` pins it.
- Indicator lag: a reader sees an entry vanish only when the server's
expiry timer fires, up to `TypingTTL` after the last keystroke. Accepted; same
as Matrix.
- Multi-process deployments do not see each other's typing. Mitigation:
documented, same as operator signals.
- Wire mirror drift between Go and TypeScript. Mitigation:
`CHANNEL_WIRE_FIELDS` with `wireKeys` and the golden wire test.
- Existing configs with an operator `typing` type that switch on `typing:
true` fail to load. Mitigation: the refusal says to remove the type; the guide
states the migration.
- `Local` grows a field and a few calls in `expire`/`scheduleExpiry`.
Mitigation: the typing store is its own type with its own tests; plimsoll lines
are checked.

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Enhancement. `docs/realtime.md` is generated from
`docs-project/entities/guides/GUIDE-realtime.md` by `just docs`. Changes: the
example replaces the `typing` ephemeral type with `typing: true`; a "Typing
indicator" subsection under "Channel types" (the key, who may send, who sees it,
TTL 6s, rate 1/s, caps, cleared on send, when readers are woken, not stored or
audited, no channel created, `signal_rate` does not apply, separate from
operator signals, an operator `typing` type is reserved when on); the two rules
in "Checks when the file loads"; the `ephemeral` row in "Message types" no
longer cites typing; "In the web app" (the composer sends it for log types,
"<name> is typing…"); "HTTP API" (`POST …/_typing`, `typing_now`,
`_actions.typing`, `typing` in the schema endpoint, the refusals); "Agents and
other processes" (`typing_now` in `read_channel`, no send tool, empty under
stdio); "Limits" (the constants); "Reloading" (toggle allowed; live entries hide
at once). `GUIDE-mcp-server.md`: the `read_channel` note. Code: godoc on the
constants, `ChannelType.Typing`, `Service.Typing`, `TypingView`, the typing
store and the wake rule.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 15 findings (4 significant, 9 minor, 2 nit), all
addressed in this plan. The wake model now marks readers only on start, on clear
by a log post and on server expiry; the wire carries `{author, since}` and
clients do no expiry. The bool check keeps yaml.v3's accepted set. Typing has
its own store with refuse-only caps. Head and Read share one split helper.
`logOnly`, the 422 row, the MCP send tool and `Channel.typing` are dropped. The
typist capability is its own interface. AC 16 covers only what the head can
express. The composer emits typing for log types only. The indicator is sorted
by `since` and lives in its own atomic region. Plural-aware text and the missing
tests (uniform 404, live frames, reload round, watch) are added.
