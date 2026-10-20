---
id: PLAN-9UAJLA
type: planning-checklist
title: 'Planning: Realtime 3/7: SPA channels'
started: "2026-10-19"
completed: "2026-10-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- realtime.yaml: optional `label` per lifecycle op; property labels served.
- `realtime.Service.Describe` and `GET /api/v1/_schema/channels`: channel types, message types, property types, required, markdown, ref targets, enum values, labels, create label, remove reason rule. Same for every principal; empty when realtime is off.
- Per-message `edit`, `delete`, `remove`, `release` in `Message._actions`, computed by the operations' own checks.
- data-entry.yaml `channels:` section (ordered fields in the `FormField` shape), validated against realtime.yaml through a consumer-side interface, served in `_config`.
- A `refresh` broadcast after a realtime.yaml reload.
- Wire types in `frontend/src/types/channels.ts` with a Go field-set check; `api/channels.ts`; `useChannel`, `useEntityChannels`, the descriptor store; `renderMessageMarkdown` (allowlist); `channelValues.ts`.
- Pure rela-components channel components with stories.
- `ChannelsPanel.vue` on the entity page; e2e for two users; docs.

Out of scope:
- `Board` and `Session` mockups (phase 4).
- Typing indicators, unread counts, thread views (D4 to D6).
- Disabled buttons with denial reasons (D3: hidden).
- The self-asserted author marker (not on the wire).
- A server-side merge or `keep` sentinel for edits with restricted refs.
- Per-channel presence; the entity presence avatars are reused.

**Acceptance Criteria:** The 23 criteria in .ignored/t7-plan.md section 15, each
with its named test. In short:
1. Descriptor endpoint content, same for every principal, empty when off.
2. data-entry.yaml `channels:` validation and serving.
3. `refresh` on realtime.yaml reload; descriptor reload; failed load hides the panel.
4. Allowlist renderer cases, world-aware entity links.
5. Restricted refs show no id, type, title or link.
6. Untrusted strings render as text.
7. Composer order, labels, fallback order, hidden fields, required, `max`, post types.
8. `useChannel` single-flight, monotonic rev, loop cap, stale answers, clear on refusal, keep on transient.
9. `useEntityChannels` clear on refusal, keep on transient.
10. Message buttons from per-message `_actions` (edit window, pending, delete versus remove).
11. Edit mode; no edit for a message with a restricted ref.
12. Removed placeholder; removers see who and why.
13. System badge.
14. Error mapping by problem type; message errors never clear the panel.
15. Ephemeral shown after 202, hidden at expiry, no reply control.
16. Lifecycle changes while open; removed message type reloads the descriptor.
17. Composer value conversion.
18. Pending promote release.
19. Request count per frame.
20. Wire field sets match.
21. E2E: two users, faced type, live post, no composer without the post role, closed channel, live revocation.
22. Stories and purity check.
23. `just ci`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. The design reuses in-repo patterns. Prior art is in
.ignored/phase3-plan.md and the untracked mockups.

**Existing Solutions:**
- Libraries: `marked` and `dompurify` (already used); no new dependency.
- Codebase patterns:
  - `frontend/src/composables/usePresence.ts`: live subscription, sequence counter, clear on a refused refetch.
  - `frontend/src/composables/useLiveSubscription.ts` and `api/live.ts`: the subscription seam; the server already accepts channel paths (`internal/dataentry/livepath.go`).
  - `frontend/src/components/entity/CommentsPanel.vue`: app wrapper around presentational components on EntityDetail.
  - `frontend/src/utils/markdown.ts`: the sanitizer pipeline and the dedicated mermaid purifier instance.
  - `frontend/src/components/forms/EntityPickerModal.vue`: ref picking, passed to the library as `pickRef`.
  - `internal/dataentryconfig` `FormField` and `acl.Policy.ValidateAgainstMetamodel`: the field shape and cross-file validation through a consumer-side interface.
  - `e2e/tests/presence.spec.ts`: two users with `X-Forwarded-User`.
  - `rela-components` comment components: checked for reuse in T7b.
  - `internal/dataentry/api_v1.go` `handleV1SchemaRoutes`: config read sub-routes.
- Reference implementations: chat UIs merge by message id and revision; the same rule as the T6 CLI watch.
- rela concepts: "The configuration is not a secret; the data is"; the row-level uniform 404 for entity existence; phase 3 security notes for the SPA.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- See .ignored/t7-plan.md sections 13 to 16 (they supersede sections 0 to 12).
- Structure from realtime.yaml through `realtime.Service.Describe`; presentation from data-entry.yaml `channels:`, mirroring schema.yaml and data-entry.yaml.
- dataentryconfig validates `channels:` against a narrow consumer-side `ChannelSchema` interface that appbuild supplies, like `acl.Policy.ValidateAgainstMetamodel`.
- Buttons come only from the server's channel and per-message `_actions`.
- Library components are pure: renderer, ref picker, enum options, entity hrefs and pending state come in as props.
- `useChannel` follows `live:changed` with single-flight `after_rev` pages and a monotonic held rev.

Alternatives rejected:
- Presentation keys in realtime.yaml: mixes structure and presentation, unlike schema.yaml and data-entry.yaml.
- Building the descriptor in dataentry from realtimecfg: arch-lint forbids it and it would drift from `messageTypes`.
- Deriving own-message buttons from the head flags: the edit window, pending state and `delete_own.from` would make them fail.
- A server-side edit merge in T7: more surface; hiding edit for restricted refs avoids the erasure.
- Reusing `renderMarkdown` or a forbid-list variant: both let `style`, `background` and images through.
- Wire types in rela-components: binds the library to rela's HTTP shapes.
- openapi-typescript: no other surface uses it yet; a field-set check covers the drift.

**Files to modify:**
- `internal/realtimecfg/{raw.go,compiled.go,load.go}` (lifecycle op label) (+tests)
- `internal/realtime/{describe.go,service.go,render.go}` (Describe, per-message actions) (+tests)
- `internal/apiwire/v1/channels.go`, `internal/dataentry/{channels_schema.go,channels_wire.go,api_v1.go,realtime_wiring.go}`, `internal/openapi/channels.go` (+tests, goldens)
- `internal/dataentryconfig/{config.go,channels.go}` (+tests), `internal/appbuild` wiring of the validation, `rela analyze` and Configure
- `frontend/src/types/channels.ts`, `frontend/src/api/{channels.ts,live.ts}`, `frontend/src/composables/{useChannel.ts,useEntityChannels.ts}`, `frontend/src/stores/channelSchema.ts`, `frontend/src/utils/{markdown.ts,channelValues.ts}` (+tests)
- `frontend/packages/rela-components/src/components/channel/*` (new, +tests, +stories)
- `frontend/src/components/entity/{ChannelsPanel.vue,EntityDetail.vue}` (+tests)
- `e2e/tests/realtime/channels.spec.ts`, `e2e/pages/channels.page.ts`, a typed `api` fixture helper
- docs: `GUIDE-realtime.md`, the data-entry guide, `docs/realtime.md`, `docs/data-entry.md`, rela-components README, `frontend/CLAUDE.md`

Dependencies:
- T3b channel HTTP routes and wire types; T4 `_actions` and lifecycle; T5 `author.system`, `triggered_by`; T6b `asserted_by`.
- `useLiveSubscription`, `shouldDropHeldContent`, `PendingButton`, `EntityPickerModal`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Message props, channel titles, removal reasons, ref titles and author names: written by members, agents and triggers; untrusted. Markdown props go only through `renderMessageMarkdown`; everything else renders as text.
- `renderMessageMarkdown`: a dedicated DOMPurify instance with an allowlist of tags (p, br, strong, em, del, code, pre, blockquote, lists, a, headings, hr, tables) and attributes (href, title), `SANITIZE_NAMED_PROPS`, and a URI allowlist (http, https, mailto). Entity links are built after sanitizing with DOM APIs, only from server-resolved ids. It is the only `v-html` in the channel components.
- Composer input: checked in the client by `channelValues.ts`; the server validates again.
- realtime.yaml and data-entry.yaml: operator-trusted config; labels render as text; `channels:` is validated against realtime.yaml.

**Security-Sensitive Operations:**
- Authorization stays on the server. Channel and per-message `_actions` are computed by the operations' own checks; the SPA shows only those.
- Restricted refs carry no id on the wire, and the SPA shows a placeholder only; a message with a restricted ref cannot be edited, so no edit erases a ref the editor cannot read.
- Existence: the descriptor endpoint carries config only. A refused head or list refetch clears held content, so a revoked reader stops seeing the channel; message-level errors do not.
- No remote fetch from message content: no images, no `style` or `background`, no mermaid or PlantUML.

Error handling:
- The SPA shows the server's problem title, mapped by problem type; it adds no local data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see .ignored/t7-plan.md section 15; every criterion names
its test.

Integration approach:
- Go tests run the handlers over a realtime.yaml and data-entry.yaml fixture; the golden and field-set files come from the real handlers.
- E2E runs the built rela-server, following `presence.spec.ts` with two `X-Forwarded-User` contexts, and writes realtime.yaml and acl.yaml per test.

**Edge Cases:**
- A `one` channel with no messages: empty list, composer shown to posters.
- A full `after_rev` page: the loop continues until `has_more` is false or the cap, then reloads the newest page.
- A frame during an in-flight fetch: one rerun; an out-of-order answer never moves the held rev back.
- An edit and a removal arriving by `after_rev`: the held message is replaced by the newer rev.
- An answer for an old target after switching channels: dropped.
- A 202 ephemeral post: the head is refetched directly; the signal hides at `expires_at`.
- A config reload that removes a message type or property under an open composer: fields re-render, removed values are not sent.
- A message whose type the config no longer declares: rendered without props.
- More than one channel type on an entity, and `many` types with several channels: tabs, one open at a time.
- A hidden tab: refetch deferred until visible.
- A faced anchor type and a page world: URLs and live paths carry face and world.
- A pending promote: status shown, Release for callers with `release`.
- Date values in a negative UTC offset; empty optional fields; a DST-gap datetime.

**Negative Tests:**
- Hostile markdown: script, `img onerror`, markdown images, `javascript:` and `data:` links, `onclick`, svg with script, iframe, style.
- `style="background:url()"`, `style="position:fixed"`, `<table background>`, `<a id=app>`, `<a ping>`, `[x](/entities/foo)`, `[x](//evil)`.
- A message with a restricted ref: no edit button, no PATCH.
- data-entry.yaml `channels:` naming an unknown channel type, message type or property, an unsupported field key, or a hidden required property: refused.
- HTML in string props, titles, reasons and ref titles: shown as text.
- A user without a post role: no composer. Without `remove`: no remove button. Another author's message: no edit or delete.
- Each problem type in `channels_errors.go`: the mapped UI (plan section 14 R12); only a refused head or list refetch clears held content.
- A refused refetch after access is revoked: the channel disappears.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
1. Sanitizer gaps on untrusted markdown. Mitigation: dedicated purifier, forbid list, URI allowlist, table test, single `v-html`.
2. Wire drift. Mitigation: the Go field-set check against the TS key list.
3. Live subscription budget (16 per stream) and request load. Mitigation: one open channel per panel; the list subscription is dropped while a channel is open; request count pinned.
4. EntityDetail size (3882 lines). Mitigation: logic stays in `ChannelsPanel.vue`.
5. Promote answers with warnings. Mitigation: toast with the titles and a link chip to the new entity.
6. Config split over two files. Mitigation: cross-file validation at load, in `rela analyze` and in Configure.
7. Large channels. Mitigation: memoized rendering, a 500-message cap, scroll handling.

Effort: l (three PRs: T7a config, endpoint and plumbing; T7b library components;
T7c panel, mount, e2e and docs).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no schema.yaml change; realtime.yaml labels are documented in docs/realtime.md)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change)
- [x] docs/data-entry.md - UI changes (the `channels:` section and the channels panel on the entity page)
- [x] CLAUDE.md - New patterns or conventions (`frontend/CLAUDE.md`: untrusted message markdown goes through `renderMessageMarkdown`)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] ~~N/A - Internal change~~ (N/A: user-facing docs are updated)

Also:
- `docs/realtime.md` via `GUIDE-realtime.md` (web app section)
- the API reference for `_schema/channels` (generated from OpenAPI)
- rela-components README (channel components, injected renderer)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 29 findings in .ignored/dr-t7.tsv (2 critical, 14
significant, 7 minor, 6 nit), all addressed in .ignored/t7-plan.md section 14;
decisions from Jeroen recorded in section 13.
