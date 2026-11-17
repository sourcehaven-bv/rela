---
id: PLAN-5EHAQXI
type: planning-checklist
title: 'Planning: Configure: edit flows.yaml triggers'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `flows.yaml` becomes a third Configure file
(`configedit.FlowsFile`). Snapshot, draft, preview, save, audit and restore
cover it. A per-key allowlist for `triggers.*` opens the keys the old
`automations.[]` allowlist opened, mapped to the new shape. The SPA gets the
Automations index and detail views back, with routes, sidebar entry, change
review, problem routing, property-rename follow-up and delete warnings. Out: the
`flows:` section (locked whole), script outputs, `on.deleted`, `on.faces`,
`on.copy`, `create.template`. Postgres and sqlite builds (Configure stays
fs-only).

**Acceptance Criteria:**
1. A `config:edit` holder edits trigger description, `on` keys (entity, property, becomes, from, created, updated, relation_created, relation_removed, where, when) and set/relate/create/add_to_pile/notify/check outputs. Test: `TestCheckEdit_Flows` open-key rows return no violation.
2. Locked keys stay read-only: script.**, on.deleted, on.faces, on.copy, create.template, flows.**. The `on` of a trigger with a script output is pinned. Test: `TestCheckEdit_Flows` locked rows; SPA models test for `readOnly`.
3. A save that breaks a trigger is refused at preview. Test: rela-server test where the draft trigger names a missing property and preview returns an `invalid` problem.
4. A save of flows.yaml writes a `config-edit` audit record naming flows.yaml by hash and the changed paths. Test: service test inspects the recorded audit summary.
5. Gate unchanged: `--config-editing`, `config:edit`, fs build only. Existing `TestConfigure_Gate` and `TestValidateConfigEditing` keep passing; no new mount.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: port of an editor removed in TKT-GWOBH2U; the approach is fixed by that code)
- [x] Searched for existing libraries that solve this problem (none needed; existing configedit tree, merge and allowlist)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: restoring our own removed editor)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- Removed editor: `git show d0764fe8d^:frontend/src/views/configure/AutomationView.vue`, `AutomationsView.vue`, `models.ts automations()`.
- Removed allowlist rows: `git show d0764fe8d -- internal/configedit/allowlist.go`.
- Trigger shape: `internal/flowcfg/triggers/parse.go` raw structs.
- Trigger check: `appbuild.CheckTriggers`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Backend:
- `configedit`: `FlowsFile = "flows.yaml"`. `current` reads it; an absent file reads as an empty mapping and is written with "must not exist" semantics. `versionOf` hashes all three files. `Snapshot.Flows`, `Draft.Flows`. `prepare` handles it like the other two (CheckEdit, Apply). `write`, `restore` and `audit` include it.
- `Validator.Validate(schema, dataEntry, flows []byte)`; rela-server checks the draft bytes with `appbuild.CheckTriggerData(flows, meta, store)` instead of reading flows.yaml from disk, so preview judges the draft.
- Allowlist mapping old to new: `on.condition` to `on.when`, `on.when.[]` to `on.where.[]`, `do.[].set/value` to `outputs.[].set.properties.*`, `create_relation.*` to `outputs.[].relate.{type,to}`, `create_entity.*` to `outputs.[].create.{type,properties.*,relate.type,relate.from,if_exists}`, `add_to_pile.*`, `notify.*`, `validate.[].*` to `outputs.[].check.{where,message,severity}`. Locked: `outputs.[].script.**`, `create.template`, `on.deleted`, `on.faces.**`, `on.copy.**`, `flows.**`. Pin `triggers.*` key `on` by `outputs.[].script.**`.
- `triggers.DecodeTypes()` exposes the rule and output reflect types so `TestAllowlist_ClassifiesEveryKey` walks flows.yaml too.

Frontend:
- `api/configure.ts`: `ConfigFile` gains flows.yaml; snapshot and draft carry `flows`.
- `configDraft`: third file `flows`, persisted, sent in the body, included in describeChanges.
- `models.ts`: `automations(flows)` keyed by trigger name; `readOnly` when a script output, template, deleted, faces or copy is present.
- Views `AutomationsView.vue`, `AutomationView.vue`; routes `/configure/automations` and `/configure/automations/:name`; sidebar entry; `changes.ts` group; `paths.ts` problem route for flows.yaml; `references.ts` renames `on.property` and `set.properties` keys on a property rename; delete confirmations mention automations. The "automations moved" callout goes.

**Alternatives rejected:**
- A separate Configure endpoint per file: duplicates version and conflict handling and lets schema and triggers save out of step.
- Reading flows.yaml from disk in Validate: would judge the old triggers, not the draft.
- A `name` key for renames: the name is the map key now; a rename is a remove plus an add, which the allowlist already judges per key.

**Files to modify:**
internal/configedit/{allowlist.go,service.go,allowlist_test.go,service_test.go},
internal/flowcfg/triggers/parse.go, internal/appbuild/schemacheck.go,
cmd/rela-server/configure.go (+test), frontend/src/api/configure.ts,
frontend/src/configure/{configDraft,models,changes,paths,references,routes}.ts,
frontend/src/views/configure/{Automations,Automation,Rules,EntityType}View.vue,
frontend/src/components/configure/ConfigureSidebar.vue,
frontend/src/router/index.ts, i18n files, docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1):** Asset: the trigger
engine runs on every write; script outputs run Lua with capabilities and ACL
bypass. Actor: a principal holding `config:edit` (trusted to shape the model,
not to run code), or an attacker who hijacks such a session.
- T1 Elevation: add or change a `script:` output (Lua, capabilities, allow_acl_bypass, background, run_as). Mitigation: `outputs.[].script.**` locked; refused with 422 on preview and save.
- T2 Elevation by repointing reviewed code: change the `on` of a trigger holding a script, so reviewed Lua runs on other records. Mitigation: `on` pinned by `script.**`.
- T3 Two-step loosening: remove a scripted trigger and add it back changed. The removal is allowed (it takes code away); the re-add carries `script` and is refused.
- T4 Template file reads: `create.template` loads a template file; locked as before.
- T5 Background jobs: `on.deleted` only starts background Lua; locked.
- T6 Tampering through YAML: anchors, aliases or `<<` in the draft. FromYAML refuses them, Apply refuses bytes that do not read back as the checked tree, and triggers.Parse refuses aliases.
- T7 Tampering with `flows:` (stateful runs, permissions, run_as): `flows.**` locked.
- T8 Repudiation: every save and refusal writes a `config-edit` audit record with the hashes of all three files and the changed or refused paths, never content.
- T9 Information disclosure: the configuration is not a secret (CLAUDE.md). flows.yaml holds no secrets; those live in `.rela/secrets.yaml`.
- T10 Denial of service: the draft size is bounded by maxDraftBytes and the tree limits. A broken trigger is refused at preview, and Prepare builds a full candidate before the swap, so the server is never replaced by one that fails to start.
- T11 Spoofing: the gate is unchanged (`config:edit`, interactive principal type, verified identity beyond loopback).
Residual: a `config:edit` holder can write data through set, create,
add_to_pile and notify outputs, which run as cascades without per-record ACL
checks. TKT-CB1YV7D, done in this ticket, judges every such edit against the
parsed acl.yaml and the editor's own grants (`appbuild.CheckTriggerACL`) and
refuses it when it cannot judge.

**Input Sources & Validation:** Draft JSON from the browser: strict JSON decode,
tree limits, per-key allowlist, read-back check, then the full trigger parse and
compile against the draft schema. Invalid input gives 400 or 422 with problems.

**Security-Sensitive Operations:** File writes through RootedFS (project root
only), compare-and-swap against the content read, restore on failure.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- Go table test `TestCheckEdit_Flows`: each open key editable, each locked key refused, pinned `on`, scripted trigger removal allowed, re-add refused.
- `TestAllowlist_ClassifiesEveryKey` covers flows.yaml.
- Service tests: a flows-only save writes flows.yaml and audits it; an absent flows.yaml is created; restore after a Prepare failure includes flows.yaml.
- rela-server integration: preview of a draft trigger naming a missing property returns an `invalid` problem; a valid trigger edit saves and the new server fires it.
- Frontend vitest: models `automations()`, configDraft flows edits, changes group, reference rename.
- e2e: the old editor had none; none added.

**Edge Cases:**
- flows.yaml absent; flows.yaml with only `flows:`; empty `triggers:`; a trigger with both script and set outputs (read-only).

**Negative Tests:**
- Adding `script:`, `create.template`, `on.deleted`, `on.faces`, changing the `on` of a scripted trigger, editing `flows:`: refused with locked problems.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Rewriting flows.yaml could change the `flows:` text and so flow versions. Mitigation: flows.yaml is only rewritten when the draft edits it; Apply keeps the layout; a flow version hashes comment-free node content.
- The version format change makes browser drafts made before a deploy stale; the UI then offers to discard, as for any outside edit.
Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- docs-project Configure guide: the Automations page edits flows.yaml triggers.
- `.claude/rules/configedit.md`: three files.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-6KYGN2F (critical); RR-HMLVLUD, RR-MPV30G0,
RR-4F2XZWO, RR-O1U5C8Z, RR-O4CN93R (significant); RR-QOY14WM, RR-EZ8FC1E,
RR-441BKLI (minor); RR-7WWYHCN (nit). All addressed except RR-7WWYHCN (wont-fix:
existing behaviour for every Configure file).
