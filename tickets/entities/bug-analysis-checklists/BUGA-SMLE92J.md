---
id: BUGA-SMLE92J
type: bug-analysis-checklist
title: 'Analysis: The Changes list shows raw actors and relation names; History opens empty'
started: "2026-11-20"
completed: "2026-11-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestChangeTitles_NamesActorsByTitle fails on develop (no user ids in the batch); the changeText cases for PSN-19, webhook:, system:webhook: and integration: fail on the old actorText; the EntityDetail empty-timeline case shows History on develop)
- [x] Minimal reproduction steps documented (in harbourmaster's back office, open an account the IdP webhook created and a person edited: the Changes list shows PSN-019, webhook:membership.created and "Completes signup of ← owner@…"; History opens "No versions recorded yet" with the subtitle "account · ACC-019")
- [x] Environment/conditions noted (database builds with an audit log; principal_property resolving people to entity ids; the IdP webhook; relations without inverse labels; an entity with no captured version yet)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (the server adds each record's user to the one title batch when it is not a reserved or webhook identity; the panel shows a user's title, names webhook and integration identities by kind and name, and reads an incoming relation without an inverse label as "Label, from Other"; EntityDetail reads the timeline once per address where History is offered and shows the button only when it has versions; HistoryView names the entity by its title)
- [x] Regression test planned (Go: TestChangeTitles_NamesActorsByTitle, TestActorMayBeEntity; frontend: changeText and ChangesPanel cases, useHasHistory tests, an EntityDetail empty-timeline case in both blocks, HistoryView subject tests)
- [x] Related areas checked for similar issues (the global audit view keeps raw values for audit:read holders on purpose; RelationCards and RelationPicker already use inverse.label; the timeline rows on the history page show the principal as sent, which is the operator-facing history and out of scope)
