---
id: PLAN-8FWEGI
type: planning-checklist
title: 'Planning: faces.<name>.messages.notice — text on a face regardless of writability'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — a second key on `FaceMessages`, `notice`, rendered on the entity DETAIL
page whenever that face is on screen, independent of `_actions`:

- `metamodel.FaceMessages.Notice` (`internal/metamodel/types.go`)
- `v1.FaceMessages.Notice` + `faceMessagesWire`
(`internal/apiwire/v1/responses.go`, `internal/dataentry/schemaworlds.go`)
- `frontend/src/types/schema.ts` face messages type
- `EntityDetail.vue`: a `noticeNote` computed and the existing
non-absent `WorldBanner`'s `v-if` and slot
- docs: `docs/metamodel.md`, `docs/content-states.md`, `docs/data-entry.md`
wherever `read_only` is already described

OUT, deliberately:

- **`DynamicForm.vue`.** Its `notEditableNote` renders only in the
*refusal* branch (`notEditable`, i.e. `_actions.update === false`), and that
branch is exactly where a writable draft face never lands. Putting `notice`
there would render it on no page that this ticket exists for. A notice above an
editable form is a real want, but it is a different surface with a different
render site; not smuggled in here.
- Widening or re-gating `read_only` in any way.
- New validation. `notice` names no other object, so there is nothing to
resolve; the placeholder allowlist is shared and already enforced by
substitution behaviour (an unknown `{name}` renders as written).
- Any list/board/badge surface. A notice is about the document you are
reading, and the collection surfaces already have `projection` and `stand_in`.

**Acceptance Criteria:**

1. An operator declaring `faces.<f>.messages.notice` sees that text on the
detail page of an entity served as face `<f>`, **while holding `update`** on it.
Test: `EntityDetail.world.test.ts`, mount with `_actions.update: true` and a
declared notice; assert `.world-banner` exists and carries the text.
2. Undeclared renders nothing, and renders no empty banner. Test: same
mount with no `messages` block; assert `.world-banner` does not exist.
3. `notice` and `read_only` both declared on a face the reader may NOT
write: both sentences render, `notice` first. Test: seed both, mount with
`_actions.update: false`, assert the banner text has `notice`'s index before
`read_only`'s.
4. Placeholders substitute from the same vars as `read_only`. Test: a
notice of `'{face} / {title}'` renders the face LABEL and the display title, not
the raw coordinate or the id.
5. The wire carries it, and a face with nothing declared still omits the
whole `messages` block. Test: extend `TestSchemaFaces_CarriesOperatorMessages`.
6. `worldAbsent` renders no notice. Test: mount the absent case with a
notice declared on the bare face; assert the absent banner renders and the
notice text does not appear.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — this adds one field alongside an existing one that
already carries every rule the new one needs. TKT-5SZG2L did the design work;
this ticket applies its established shape.

**Existing Solutions:**

The prior art is TKT-5SZG2L ("World chrome speaks the operator's words or stays
silent"), which introduced `WorldMessages`, `FaceMessages`, `ChromePlaceholders`
and `worldText`. Everything this ticket needs already exists:

- `metamodel.FaceMessages` (`internal/metamodel/types.go:387`) — the
struct to extend; `ReadOnly` is its only field today, which is precisely the
gap.
- `faceMessagesWire` (`internal/dataentry/schemaworlds.go:173`) — compares
against the zero struct to decide whether to emit the block, so a new field
participates in "nothing declared omits the block" for free.
- `worldText` (`frontend/src/utils/worldText.ts`) — the substitution
helper, already returning `''` for an undeclared template, which every caller
treats as "render nothing".
- `WorldBanner` (`frontend/src/components/common/WorldBanner.vue`) — the
layout component, already shared by EntityList and EntityDetail, whose doc
comment states the caller owns the `v-if`.
- `readOnlyNote` (`EntityDetail.vue:948`) — the computed to sit beside,
including its `textVars` source.

No library involved; this is config plumbing through three layers.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Mirror `ReadOnly` at each of the three layers, changing only the gate.

1. **metamodel** — add `Notice string \`yaml:"notice,omitempty"\``to`FaceMessages`, with a doc comment stating the distinction that
justifies a second key: `read_only`is about the READER (they may not
write),`notice` is about the DOCUMENT (it is not in force), so a face that is
writable can still carry one. Note which placeholders it substitutes (all four;
a detail page has every fact).

2. **wire** — add `Notice string \`json:"notice,omitempty"\``to`v1.FaceMessages`and copy it in`faceMessagesWire`. The zero-struct
comparison in that function needs no change.

3. **SPA** — widen the type to
`messages?: { read_only?: string; notice?: string }`, add

   ```ts
   const noticeNote = computed<string>(() => {
     if (worldAbsent.value || !servedFace.value) return ''
     return worldText(typeDef.value?.faces?.[servedFace.value]?.messages?.notice, textVars.value)
   })
   ```

— identical to `readOnlyNote` minus the `mayUpdate` term, which IS the feature.
Then the existing non-absent banner takes it:

```
v-if="!worldAbsent && ((isWorldBound && worldBanner) || noticeNote || readOnlyNote)"
```

with both notes in the slot, `notice` first, each in its own element so two
sentences do not run together as one string.

**Ordering.** `notice` before `read_only`: the document's status is the larger
fact and the reader's permission is a qualifier on it. The ticket allows either
order provided it is defined; defining it in a test is the point.

**`worldAbsent`.** Both notes stay silent. There is no face on screen when the
world resolves to none — the page falls back to the bare face and the absent
banner above already says so in the world's words. A per-face notice would be
attributing text to a face the reader did not reach.

**Alternatives rejected:**

- *Widen `read_only` by dropping its `mayUpdate` guard.* Changes the
meaning of every `read_only` an operator has already written, and conflates two
different sentences. Recorded in the ticket body.
- *A boolean `always: true` modifier on `read_only`.* Keeps one key but
makes its NAME wrong for half its uses, and an operator wanting both sentences
still cannot have them.
- *Reuse `worlds.<w>.banner`.* Per-world, and a fallback chain serves
several faces through one world. Recorded in the ticket body.
- *A new component beside `WorldBanner`.* Two stacked banners for one
page; the existing one already accepts a note slot.

**Files to modify:**

- `internal/metamodel/types.go`
- `internal/apiwire/v1/responses.go`
- `internal/dataentry/schemaworlds.go`
- `internal/dataentry/schemamessages_test.go`
- `internal/metamodel/copydef_unmarshal_test.go` (round-trip coverage)
- `frontend/src/types/schema.ts`
- `frontend/src/components/entity/EntityDetail.vue`
- `frontend/src/components/entity/EntityDetail.world.test.ts`
- `docs/metamodel.md`, `docs/content-states.md`, `docs/data-entry.md`

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

One input: `notice`, a plain string from `schema.yaml`. That is
operator-authored config, which per CLAUDE.md's "the configuration is not a
secret" rule is already-disclosed content, not attacker input. It is rendered
through Vue's text interpolation (`{{ }}`), which escapes — the value never
reaches `v-html`, exactly as `read_only` does not. Placeholder substitution is
an allowlist (`ChromePlaceholders`), one pass, so a substituted value is never
re-scanned; an unrecognised `{name}` renders literally rather than vanishing.

**Security-Sensitive Operations:** none. This adds no gate, reads no entity
data, and makes no authorization decision. It is deliberately *independent* of
`_actions` — but that is a display property, not an access one: the banner does
not grant or reveal anything, and the face it describes was already resolved and
served by the existing read path.

A note on what it does NOT do: `notice` must not become a way to say "this
entity is hidden" or otherwise report per-principal state. It is declared per
FACE in config, identical for every reader of that face, so it carries no
principal-dependent fact and cannot become an oracle.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1–AC6 above each name their mount and assertion. Go-side:
`TestSchemaFaces_CarriesOperatorMessages` (AC5) covers the wire end to end
through the real schema handler, not a hand-built struct. Frontend:
`EntityDetail.world.test.ts` mounts the real component with a seeded schema
store, so the computed, the `v-if` and the slot are all exercised together
rather than unit-testing the computed alone.

**Edge Cases:**

- Notice declared, `read_only` not, reader may not write → notice alone.
- `read_only` declared, notice not → unchanged from today (regression).
- Both declared, reader MAY write → notice alone (`read_only` still gated).
- Notice declared on the BARE face, page not world-bound → renders; a bare
face is a face, and the ticket's rule is "whenever that face is on screen".
- `worldAbsent` with a notice on the bare face → no notice (AC6).
- Notice `'  '` (whitespace only) → `worldText` returns it truthy, so the
banner renders a blank note. Accepted: an operator who typed spaces gets spaces,
consistent with "the text is the operator's".
- `{face}` on a face with no `label` → substitutes the empty string, per
`worldText`'s documented "supplied as `''` substitutes to nothing".
- Empty `messages: {}` block in YAML → zero struct → block omitted on the
wire, nothing renders.

**Negative Tests:**

- No `messages` block: `faceMessagesWire` returns nil, `.world-banner`
absent. Both sides asserted.
- An unknown placeholder `{nope}` renders literally (already pinned for
the shared helper; not re-asserted per key).
- `TestChromePlaceholdersInSyncWithFrontend` must still pass — this
ticket adds no placeholder, so the allowlist is untouched, and that test failing
would mean something went wrong.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Regressing `read_only`* by editing the shared `v-if`. Mitigated by
keeping the existing tests untouched and adding new ones beside them; a broken
gate fails the TKT-5SZG2L assertions immediately.
- *Two notes rendering as one run-on sentence.* Mitigated by separate
elements in the slot rather than string concatenation.
- *Wire block emitted for a face with nothing declared.* Guarded by the
existing zero-struct comparison and the existing assertion that a bare face
omits the block.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/metamodel.md` — the face `messages:` table gains a row
- [x] `docs/content-states.md` — the chrome step that introduces
`read_only` gains its sibling, with the reader/document distinction
- [x] `docs/data-entry.md` — worlds section, where face chrome is listed
- [x] ~~`docs/cli-reference.md`~~ (N/A: no command changes)
- [x] ~~`CLAUDE.md`~~ (N/A: no new pattern; this follows TKT-5SZG2L's

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one
additive config field following an established in-tree pattern, with no new
validation, gate or component; the design questions — why not widen `read_only`,
why not a new component, ordering, the `worldAbsent` rule — are settled and
recorded above)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
