---
id: RR-TXT57E
type: review-response
title: D1 frontend change is three edits across two files + a test, not 'one line'; and invert (don't just add) the view-deferral tests + fix stale godoc
finding: |-
    Two accuracy corrections to the plan, neither blocking but both worth fixing so implementation doesn't under-scope.

    1. FRONTEND is not 'one line'. CommandModal.vue receives only entityId (props L8-10) and has no type. Sending entity_type requires: (a) add an entityType prop to CommandModal, (b) params.set('entity_type', props.entityType) at L42-45, (c) pass :entity-type from the caller EntityDetail.vue:1191 (EntityDetail has entityType at props L54, so it's available), plus (d) update CommandModal.test.ts URL assertions (L83/127). Three edits across two files + a test. GOOD NEWS confirmed by grep: CommandModal.vue is the ONLY frontend caller of POST /api/command/ — no frontend sends list_id or view_id, and auto_open (L106) is output-file handling, not an invocation. So D1's hard-400 breaks exactly one SPA path, fixable in that one component; the plan's sequencing (fold frontend into this PR) is sound. State that the search was done rather than assume no server-side auto-invoke path constructs an entity URL (none found).

    2. VIEW-DEFERRAL TESTS must be INVERTED, not just added. TestCommandExecDeclarativeFailsClosed currently asserts view is DENIED-despite-granted-permission (the MJ02AO deferral). After D3 that must FLIP to granted (200). The plan's Test Plan adds the new 'view granted' row but does not say to delete/invert the old 'view denied despite permission' case — leave it and it rots green against stale behavior or fails. Also update the stale in-code godoc documenting the deferral as intentional (authorizeCommand comment block and the view-arm comment in commands.go) — the plan mentions docs regen but not these in-code comments.

    3. D1 400 must return BEFORE any store read (it does in the plan's step-1 flow) so the error is not an oracle; 'entity_type is required' is a safe body (no entity data).
severity: minor
resolution: Folded into PLAN-Z2OIV7. (1) Frontend corrected from 'one line' to three edits across two files + a test (CommandModal.vue prop + params.set, EntityDetail.vue :entity-type pass-through, CommandModal.test.ts URL assertions); the grep confirming CommandModal.vue is the only frontend /api/command/ caller is recorded so the D1 hard-400 blast radius is known. (2) The view-deferral test must be INVERTED not just added — the plan's Approach and Test Plan now say to flip the existing TestCommandExecDeclarativeFailsClosed 'view denied despite granted permission' case to assert 200, and to update the stale in-code godoc (authorizeCommand block + view-arm comment). (3) The 400-before-any-store-read ordering (so the missing-entity_type error is not an oracle) is pinned in both Approach and the Test Plan.
status: addressed
---
