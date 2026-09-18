---
id: IMPL-AHUNK3
type: implementation-checklist
title: 'Implementation: Create related entities from the entity detail page (section + header buttons, modal or page flow)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Go: `validate_sectioncreate_test.go` (config shape, opt-in default, scalar
refusal, ambiguous-section refusal, per-type templates on a heterogeneous
relation), `sectioncreate_test.go` (wire affordance, both link directions,
template resolution, header union + dedup, ACL gating over two principals),
`acl_sidepanel_test.go` (the side panel's new gate), `querybudget_test.go` (zero
added store reads), and the existing `TestV1Views_NoAddOrLinkInfoOnSections`
extended to assert `create` is absent without opt-in.

Frontend: `createLink.test.ts`, `SidePanel.test.ts` (the param-mismatch fix,
asserted against the names the form parses), `SectionCreateButton.test.ts`,
`EntityDetail.create.test.ts` (the no-double-link regression),
`DynamicForm.embedded.test.ts` (prop channel, both directions, world, surfaced
link failure), `InlineCreateFormModal.test.ts` (prop forwarding).

Error handling: an unresolvable `link_peer` and a failed link both surface a
toast naming the created id. The previous code skipped the link silently.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Builders: `sectionCreateConfig` / `validateOneView` (Go config),
`seedCreateView` / `fetchView` (Go handler), `createAffordance` /
`viewWithCreateSection` (Vue). Assertions compare against the fixture object —
e.g. `emitted[1]).toEqual(create.targets[0])` rather than restating the literal.

**Mutation verification.** Every test whose value depends on catching a specific
defect was checked by reverting the fix and confirming it fails:

| Test | Reverted | Result |
| --- | --- | --- |
| `TestSectionCreate_GatedByCreatePermission` | removed the ACL check | 2 subtests fail |
| `SidePanel.test.ts` (both) | restored `_relation`/`_linkAs`/`_peerId` | both fail |
| `DynamicForm.embedded` "links via the prop" | removed the `pickerTypes` registration | create never fires |
| `DynamicForm.embedded` from-direction (both) | restored the inverted call | both fail |
| `EntityDetail.create` no-double-link | re-added the host's `createRelation` | fails |

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` against `prototypes/data-entry/project` (real operator config,
not a fixture) with `create:` added to the Tickets section — an INCOMING
`belongs-to` traversal, so the harder direction. Drove the SPA with a real
browser.

| AC | Evidence |
| --- | --- |
| 1 | `_views` as `alice@example.com` (editor) carried the affordance on Tickets only; as `bob@example.com` (viewer, no `create`) every section returned `create: null` and the header menu was absent |
| 2 | One reachable type rendered a direct `+ Ticket` button, not a menu |
| 3 | Modal opened over the page, `location.pathname` unchanged, 10 fields, view refetched on success, new ticket visible in the section |
| 4 | Page flow navigated to `/form/create_ticket` carrying `link_relation=belongs-to`, `link_peer=backend`, `link_as=from`, `return_to=/entity/category/backend`, `template=bug`; on submit returned to the category with TKT-007 present |
| 5 | Exactly ONE relation file on disk: `TKT-007--belongs-to--backend.md` with `from: TKT-007, to: backend`. No duplicate, no backwards edge |
| 6 | The "Bug" template pill rendered `active` and the created entity's body was the bug template |
| 7 | Header menu carried one entry labelled "Tickets"; absent for the viewer |
| 9 | Putting `create:` on "Blocked Tickets" (fed from the `tickets` bucket, not `entry`) refused startup: *view "category": section[2] declares create but no single relation fills it* |
| 13 | Side-panel Add button present for a principal with `create`, absent without |

Two defects were found here that no unit test had caught, both now fixed and
covered (RR-X3O9RR, RR-Q99JT6). Both lived in the pre-link seam, which had no
live caller before this ticket because the side panel's params were dead.

A false negative worth recording: the affordance first appeared absent in the
browser after a rebuild. That was a stale cached bundle, not a regression —
confirmed by the API returning the affordance correctly at the same moment.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `creatableTargets` is the one gated derivation both add
surfaces use; `SectionCreateTarget` replaced the near-identical
`SectionAddTarget` rather than sitting beside it; `buildCreateLinkQuery`
centralises the query names that had drifted; one `SectionCreateButton` serves
all six display branches rather than a copy per branch.

Deliberately NOT extracted: `resolveSectionCreate` is kept separate from
`resolveSectionButtonsWithTraverse` even though they overlap, because the latter
also builds an ungated `LinkInfo` that is out of scope here (RR-DZJACK).

Security: targets derive from `computeCollectionActions`, the same call the list
handler uses; the write path re-authorizes independently. The plan's claim of
server-sent provenance was withdrawn as untrue rather than left in (RR-V32AZK).
