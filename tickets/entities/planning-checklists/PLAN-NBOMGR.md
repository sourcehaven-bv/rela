---
id: PLAN-NBOMGR
type: planning-checklist
title: 'Planning: E2E fixture and test matrix for faces and worlds'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope (Stage 0 of RES-Y6JA37): a faced e2e fixture project, the Playwright
harness that starts `rela-server` on it as a chosen principal, page objects for
the face and world affordances, live specs for flows that work today, and
`test.fixme` specs for flows tracked by backlog bugs.

Out of scope: product code changes of any kind; fixing the bugs the fixme specs
name; the Go-side fixture swap (`facedApp` / `seedDeclaredFaceTicket` as the
default, retiring `seedDraftAndPublishedTicket`). That swap is Go test code in
`internal/dataentry` and belongs with the later stages that touch those
handlers; it is reported as a follow-up.

**Acceptance Criteria:**

1. A faced fixture project exists (`e2e/tests/faced-project.ts`): `policy` with faces `draft`/`published` and a `file` property; faceless `control` related by a content-scoped (`implements`) and an identity-scoped (`owned-by`) relation, plus a faceless-to-faced relation (`mitigates`); worlds `published` and `editorial` with `default_world: published` and `editorial.create: draft`; a `cat` transform; an anchored document; a publish copy; comments; `acl.yaml` with an all-faces `editor` (writes `policy@draft` only) and a `policy@published` `reader`.
2. `facedTest` (`e2e/tests/faced-fixtures.ts`) spawns the server on that project as a principal chosen with `test.use({ facedUser })`; a `facedPgTest` variant does the same on the postgres backend and seeds through the API.
3. Live specs pass locally and in CI: browse in the default world, `?world=editorial`, open `ID@face`, face switcher, edit a face, create into a face, publish, single-face delete, reader grant hides the draft in list and detail.
4. Fixme specs exist for BUG-CTUW2N (attachments, publish sharing, export), BUG-8J3LSB (documents), BUG-BZQQDP / BUG-FYEEVX (duplicate, relation to faced target, comments panel), BUG-4SYAA6 (history/restore, postgres), BUG-1YN750 (family delete). Each names its bug id.
5. Existing e2e specs still pass (smoke run); `npm run lint` and `npm run typecheck` in `e2e/` pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: RES-Y6JA37 already covers the programme; this unit is test scaffolding)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo Playwright harness is the reference)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-Y6JA37 (programme), DEC-NPZICR.

**Existing Solutions:**

- Playwright is already the e2e framework; no new dependency.
- `e2e/tests/fixtures.ts` owns server spawning and the `postgresTest` variant; `read-only-mode.spec.ts` shows a spec-level `test.extend` that respawns the server with other flags.
- `prototypes/worlds/project` is the model for the faced schema, worlds, copies and ACL; `docs/content-states.md` documents the wire and UI behaviour asserted.
- Page objects in `e2e/pages/` (`list`, `entity`, `form`, `comments`, `history`) are reused and extended.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `faced-project.ts`: pure data plus `writeFacedProject(dir)`; no Playwright import, so it can be run with `node` to materialise the project for manual probing. Seed rows live in files (`POL-1@draft`, `POL-1@published`, `POL-2@draft`, three controls, content edges per face, one identity edge). Constants (`FACED_SEED`, `WORLD`, `FACE`, `FACED_USERS`) are exported so specs never hardcode strings.
2. `faced-fixtures.ts`: `facedTest = test.extend(...)` overriding `testProject` (faced project) and `serverUrl` (spawn with `RELA_DATAENTRY_USER` = the `facedUser` option, default editor). A `facedApi` fixture adds typed helpers the base `api` lacks: `createPolicy(face, ...)`, `getPolicy(address, world?)`, `policyFaceExists(id, face)`, `invokeCopy`, `listComments(address)`. `facedPgTest` extends `postgresTest` the same way and seeds its rows through `facedApi`, because pgstore does not read the fixture files.
3. `fixtures.ts`: export `spawnServer`, `waitForExit` and `createTestDir` style helpers needed by the faced fixture (no behaviour change).
4. `pages/faces.page.ts`: face switcher, copy button, world banner, stand-in badge, documents panel, export menu, attachments widget selectors.
5. Specs: `faces-browse.spec.ts`, `faces-write.spec.ts`, `faces-reader.spec.ts` (live); `faces-backlog.spec.ts` (fixme, grouped by bug id), `faces-history.spec.ts` (postgres, fixme).

**Alternatives rejected:** adding faces to the shared `METAMODEL_YAML` would
change what 50 existing specs read (a declared world changes list results); a
second Playwright project in the config would run every spec twice. Seeding
everything through the API on fs is slower and exercises create paths the fixme
specs cover.

**Files to modify:**

- `e2e/tests/fixtures.ts` (export helpers)
- `e2e/tests/faced-project.ts`, `e2e/tests/faced-fixtures.ts` (new)
- `e2e/pages/faces.page.ts` (new), `e2e/pages/index.ts`
- `e2e/tests/faces-*.spec.ts` (new)
- `e2e/tests/AGENTS.md` (document the faced fixture)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Test-only code. Inputs are constants in the fixture. The principal is chosen by
`RELA_DATAENTRY_USER`, the same mechanism the base fixture uses.

**Security-Sensitive Operations:**

The reader specs assert ACL behaviour (a `policy@published` grant hides the
draft in list and detail with the uniform not-found). The document fixme asserts
BUG-8J3LSB's disclosure is closed once fixed. The transform and document
commands are `cat`, run through the existing cmdexec sandbox.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- Browse: `/list/policies` lands in `published`, shows POL-1 under its published title, omits POL-2, shows the "Published view" banner. `?world=editorial` shows both, with draft titles and the editorial banner. Row links carry the world.
- Detail: `/entity/policy/POL-1@draft` shows the draft title and body; the face switcher "View Published" lands on the published face.
- Edit: Edit on the draft opens `/form/policy/POL-1@draft`; saving changes the draft only.
- Create: "+ New" from the published list opens the form in `editorial`; the new policy exists as a draft only.
- Publish: Publish on POL-2 creates `POL-2@published`; it then appears in the published list.
- Delete one face: deleting POL-1's draft from its detail page leaves the published face.
- Reader: list shows POL-1 only; `POL-2` and `POL-1@draft` render not-found; no Edit or Publish; `?world=editorial` is empty.
- Fixme: per-face content edges in the detail view and on the faceless target (untracked defect found while probing; see RR-OP6LGD).
- Fixme: attachments on a face and across publish, export, anchored document (editor renders draft; reader cannot), comments panel, duplicate, relation create to a faced target, history/restore, family delete through a detail-page Lua action (`actions/retire.lua` calls `rela.delete_entity` on the bare id, the only web-app route to a family delete).

**Edge Cases:**

- Draft-only entity in the published world (absent banner, still reachable by address).
- Content-scoped edges differ per face; identity-scoped edge shared.
- Principal without a world grant asking for that world.

**Negative Tests:**

- Reader cannot reach the draft by address, by list or by world.
- Editor has no Edit on the published face (no grant names it).
- Family delete by a draft-only deleter must not remove the published face (fixme).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Flake from SSE reloads or async list refresh: use `expect.poll` and `toBeVisible` retries, not sleeps.
- Seed files drifting from the on-disk format (e.g. `from_face` in relation frontmatter): the fixture is exercised by every faced spec, so drift fails loudly.
- CI has no pandoc: the transform is `cat`, which every runner has.
- A live spec may expose a real product bug: mark it fixme with evidence and report it, do not fix product code here.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, test-only change)

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed. `e2e/tests/AGENTS.md` gains a section on the faced fixture.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-OP6LGD (content edges leak across faces: live
spec marked fixme, reported as untracked), RR-2SPWF7 (family delete reached
through a detail-page Lua action), RR-1GNK1B (facedPgTest passes the user and
seeds through the API), RR-RIUXAP (fixme bodies assert the fixed behaviour),
RR-B5HOUM (comments panel: BUG-FYEEVX fixme), RR-36JMWD (no `?world=default`
assertions). All addressed in this plan.
