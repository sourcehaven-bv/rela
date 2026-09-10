---
id: IMPL-9MLDCA
type: implementation-checklist
title: 'Implementation: faces.<name>.messages.notice — text on a face regardless of writability'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (full flow, not just units)
- [x] Feature implemented
- [x] All edge cases from planning handled

**What was built:**

`faces.<name>.messages.notice`, mirrored through three layers:

- `internal/metamodel/types.go` — `FaceMessages.Notice`, with a doc comment
recording why it is a second key rather than a relaxed guard on `read_only` (who
the sentence is about: reader vs document).
- `internal/apiwire/v1/responses.go` — `v1.FaceMessages.Notice`.
- `internal/dataentry/schemaworlds.go` — one field added to
`faceMessagesWire`; the zero-struct comparison needed no change, so "a face with
nothing declared omits the block" still holds.
- `frontend/src/types/schema.ts` — the messages type widened.
- `frontend/src/components/entity/EntityDetail.vue` — `onScreenFace` and
`noticeNote` computeds, the existing non-absent `WorldBanner`'s `v-if` and slot,
and a `.banner-note` rule so two sentences stack.
- Docs: `docs/metamodel.md` (table row + prose + example),
`docs/content-states.md` (both mentions), `docs/data-entry.md`.

**One design decision beyond the ticket.** `readOnlyNote` gates on `servedFace`
being non-empty, which means NON-BARE — `refFace` returns '' for a
bare-addressed row. Reusing that gate for `notice` would have made the feature
inert in the canonical setup, where the draft IS the bare face. `noticeNote`
therefore resolves through `onScreenFace` (`servedFace || typeDef.bare_face`),
the same resolution `textVars.face` already used, so a note and the `{face}`
inside it always name the same face. Pinned by mutation (see below).

## Manual Verification

- [x] Feature tested end-to-end
- [x] Each acceptance criterion verified
- [x] Verification evidence documented

**Verification evidence:**

The six acceptance criteria are each covered by a test in
`EntityDetail.world.test.ts` (AC1-AC4, AC6) and `schemamessages_test.go` (AC5),
all passing. Rather than rely on tests that passed first try — the suite's own
comments warn that absence assertions pass against a crashed mount — each
load-bearing decision was **mutation-verified**, restoring the source after
each:

| Mutation | Result |
| --- | --- |
| Gate `noticeNote` on `mayUpdate` (make it behave like `read_only`) | 2 failed — the feature itself is pinned |
| Drop the bare-face fallback from `onScreenFace` | 2 failed — the bare-face decision is pinned |
| Swap the two notes' render order | 1 failed — ordering is defined, not incidental |
| Drop the `worldAbsent` guard | 1 failed — the absent rule is pinned |
| Drop `Notice` from `faceMessagesWire` | `TestSchemaFaces_CarriesNoticeIndependentOfReadOnly` failed |

The `worldAbsent` test also carries a positive control (the absent banner must
still render its own text), so its negative assertion is a statement about the
notice rather than about a blank page.

Full suites: **Go `./...` all pass**; **frontend 2452 tests / 151 files pass**.

**Live end-to-end run.** Tests alone do not prove an operator gets the
banner, so the feature was driven in a real browser against a real server:
`prototypes/worlds/project` copied to a scratch dir (the committed
prototype was NOT modified), a `notice` added to the `policy` type's
`draft` and `published` faces, SPA built, `rela-server` run as the
`editor` principal, POL-001 created.

1. **The wire** — `GET /api/v1/_schema` returned `messages.notice` for
   `draft`, and BOTH `read_only` and `notice` for `published`.
2. **The case the ticket exists for** — the draft detail page rendered
   "Let op: Draft van Access Control Policy is nog niet vastgesteld."
   *beside a live Edit, Publish and Delete button*. That is the page
   `read_only` can never mark. Placeholders substituted correctly
   (`{face}` → the label "Draft", not the coordinate; `{title}` → the
   display title). The face was the BARE one, so this is exactly the
   scenario that would have been inert had `read_only`'s gate been reused.
3. **Both keys together** — after Publish, the published face rendered
   "Vastgesteld beleid." on the first line and "Bewerken doe je in het
   concept." on the second, with no Edit button. Order correct, and the
   two sentences stacked rather than running together, confirming the
   `.banner-note` rule.
4. **Coexistence with the world banner** — the world's own `banner:`
   announcement ("Editorial — drafts included") rendered alongside the
   notice rather than being displaced.

Scratch project and server torn down afterwards; `git status` confirms
only the eleven intended files are modified.

## Quality

- [x] Code follows project patterns
- [x] No silent failures (errors surfaced, not just logged)

**Notes:**

Follows TKT-5SZG2L's established shape at every layer rather than inventing one:
same optional-string field, same nil-block wire projection, same `worldText`
substitution with the shared `ChromePlaceholders` allowlist, same "undeclared
renders nothing, no rela-authored fallback" rule, same `WorldBanner` component.

No silent-failure surface exists here: the feature has no error path. An
undeclared key rendering nothing is the specified behaviour, not a swallowed
failure, and an unknown `{placeholder}` renders literally by design so a typo is
visible on screen rather than vanishing.

Gates run locally, all clean:

- `just lint` — 0 issues (one `misspell` finding of mine fixed:
"synthesise" → "synthesize")
- `just arch-lint` — OK, no warnings
- `just comment-lint` — no unresolvable doc links across 13975 comments
- `just plimsoll` — clean
- `just coverage-check` — package and total thresholds PASS (79.4%)
- `npm run typecheck` — clean
- `npm run lint` — 0 errors; the warnings in the two touched frontend
files are pre-existing (`max-lines`, `v-html`, and two
`consistent-type-assertions` on unrelated copy-landing tests; the repo-wide
count is 49 before and after)

`npm run format:check` reports 169 files, identical before and after this change
— a pre-existing repo-wide state, deliberately not "fixed" here since
reformatting 169 files would swamp the diff.
