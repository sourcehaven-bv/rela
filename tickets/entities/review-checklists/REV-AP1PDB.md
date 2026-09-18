---
id: REV-AP1PDB
type: review-checklist
title: 'Review: Create related entities from the entity detail page (section + header buttons, modal or page flow)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Full `just ci` passes, exit 0 — that is the authoritative run and includes the
docs gate the cheaper targets skip.

Go: all packages pass. Frontend: 2801 tests across 179 files. `just lint` 0
issues; `just arch-lint` OK; `just plimsoll` clean; `just comment-lint` clean
across 15,064 comments; `just lint-md` 0 issues. Coverage 79.9%, both thresholds
PASS.

Lint found four things in my own diff, all fixed rather than suppressed: three
British spellings, and `handleV1CreateEntity` crossing the 60-statement `funlen`
limit. That function was already at 133 lines on `develop`, so the new gate
tipped it; resolved by extracting `gateCreateRelationAffordances` and
`writeCreateRelations`, both of which are coherent units rather than
length-driven splits. No new suppression was added anywhere in this ticket.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two independent reviewers (general code quality + rela's security invariants),
each verifying claims against the code rather than the plan.

**Review Responses:** 18 total — 6 critical, 8 significant, 4 minor. All
critical and significant are `addressed`; one minor is `deferred` with a reason.

| ID | Sev | Finding |
| --- | --- | --- |
| RR-0A83G4 | critical | Plan reversed TKT-651W's read-only invariant unknowingly; escalated, became opt-in |
| RR-YDYLZ5 | critical | Section button resolver had no ACL gate at all |
| RR-X3O9RR | critical | `link_as` inverted between server and form: backwards edge, prefix-less ids unlinkable |
| RR-EO300Y | critical | Relation affordance gate missing on the create path — a 403'd edge was writable |
| RR-AZ62K9 | critical | Relation-create body target never read-gated: existence oracle |
| RR-7Z3SFC | critical | Pre-linked edge silently droppable between `relations.value` and the payload |
| RR-V32AZK | significant | Plan claimed server-sent provenance; withdrawn as untrue |
| RR-8SP2UG | significant | Pre-link failed open to silence on an unresolvable peer |
| RR-XH2DD3 | significant | Worlds/faces unaddressed on the one world-capable surface |
| RR-DZJACK | significant | Generalizing the resolver would ship an ungated link affordance |
| RR-YNZBKN | significant | AC10 would have passed vacuously |
| RR-Q99JT6 | significant | `pickerTypes` gap aborted the entire create |
| RR-4IB3OQ | significant | Full-view refetch mislabelled; no budget test; repeated form sorting |
| RR-AM48JH | significant | `ViewAddInfo` is the forbidden carrier; `App` at its plimsoll cap |
| RR-19AU91 | minor | Bool-or-mapping YAML precedent missed (`DarkMode`) |
| RR-OIE28X | minor | Six cleanups: guard-test gap, stale memo comment, duplicate call, YAML error, nested rows |
| RR-RDPC48 | minor | **Deferred**: zero-face affordance over-offers under a non-default world |
| RR-WGNYNY | significant | `docs/data-entry.md` is generated; my hand-edit was discarded and failed the docs-check gate |

The deferral is argued, not convenience: it is a pre-existing shortcut in a
shared affordance helper, it fails toward less access (button shows, write
403s), and fixing it properly means making the collection-verb translation
face-aware across every create surface. Recorded with the BUG-Y0GNSB precedent
so it can be picked up as its own ticket.

Three of the criticals were found by *me* rather than by a reviewer — two during
manual verification, one while writing the AC10 test the plan had promised. That
is the argument for doing both.

RR-WGNYNY came from the full `just ci` run, after `lint`, `test`, `arch-lint`,
`plimsoll`, `comment-lint`, `lint-md` and `coverage-check` had all passed: the
docs I wrote were in a GENERATED file and would have been deleted by the next
`just docs`. Worth stating because it is the one gate the cheaper targets do not
reach.

**Unrelated changes:** none. The diff touches only the files in the plan's file
list plus `write_handler.go` and `acl_write_test.go`, both added deliberately to
close RR-EO300Y and RR-AZ62K9. The prototype project was edited for manual
verification and fully reverted (`git checkout prototypes/`).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
| --- | --- | --- |
| 1 | PASS | `TestSectionCreate_OptInEmitsAffordance` + `_AbsentWithoutOptIn` + `_GatedByCreatePermission`; guard test's five cases still assert absence. Browser + API verified for editor vs viewer |
| 2 | PASS | `SectionCreateButton.test.ts` — direct button for one target, menu for several; browser showed `+ Ticket` |
| 3 | PASS | `EntityDetail.create.test.ts` (refetch, no route change); browser: modal opened over the page, path unchanged, row appeared |
| 4 | PASS | Browser: navigated with all five params, returned to the category with the new row present |
| 5 | PASS | `DynamicForm.embedded.test.ts` both directions; on disk exactly ONE correct relation file, no duplicate, no backwards edge |
| 6 | PASS | `TestSectionCreate_PerTypeTemplateIsResolvedServerSide`; browser showed the "Bug" pill active and the created body used that template |
| 7 | PASS | `TestSectionCreate_HeaderMenuIsUnionOfOptedInSections` (incl. dedup); browser showed one header entry |
| 8 | PASS | `SidePanel.test.ts`, mutation-verified — both cases fail against the old param names |
| 9 | PASS | `TestSectionCreate_ValidationRejectsBadValues`, `_UnmarshalRejectsScalars`, `_RejectsTypeWithNoForm`, `_RejectsSectionWithNoSingleRelation`; real config refused startup with the section named |
| 10 | PASS | `TestSectionCreate_EdgeRefusedOnBothLinkDirections` — both directions against a **declared** non-creatable verdict, plus a positive. This is the AC the plan warned would pass vacuously; writing it is what found RR-EO300Y |
| 11 | PASS | `DynamicForm.embedded.test.ts` surfaced-failure case; the false-positive toast this produced in the browser is what led to RR-X3O9RR |
| 12 | PARTIAL | The world reaches the create on both flows (`_embedded.test.ts` world cases; the page flow carries `?world=`). The **affordance** still computes with a zero face, so it over-offers for a faced type under a non-default world — RR-RDPC48, deferred, no wrong-face write possible |
| 13 | PASS | `TestACLSidePanel_AddButtonFollowsCreatePermission`, both arms |
| 14 | PASS | `TestQueryBudget_ViewSectionCreateAddsNoStoreReads` — count flat at 10 and 50 rows, pinned to the same constant as the plain view, with an anti-vacuity guard that fails if the affordance did not resolve |

AC12 is the only one not fully PASS, and the gap is an over-offer rather than a
wrong write. Called out rather than rounded up.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-ZICH7Y

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Five commits, each scoped to one concern, each stating the reasoning rather than
the mechanism — in particular why this narrows TKT-651W rather than reversing
it, and why two bugs that masked each other had to be fixed together.

No TODO or FIXME added. Every probe file written during investigation was
removed (`protoverify`, `roundtrip_probe`, `roacl_probe`, `ac10_probe`,
`peer_probe`); the three worth keeping were promoted into real tests with
explanations of what they guard.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates on the ticket already being `done` and validating clean, so this item cannot be satisfied before the transition it precedes — see the note below and TKT-UFV01M)
