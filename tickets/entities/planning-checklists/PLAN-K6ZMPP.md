---
id: PLAN-K6ZMPP
type: planning-checklist
title: 'Planning: Webhook principals get principal_type webhook; acl map --webhook with --expect CI gate'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- Declarative webhooks (`POST /hooks/<id>`): the hook principal
`system:webhook:<id>` carries the verified `principal_type: webhook`.
- One shared constructor for that principal, used by the HTTP handler and by
the CLI report, so the two cannot drift.
- `rela acl map --webhook <id>`: the map for the hook principal with its
runtime ceiling, plus named permissions and relation writes, plus what the hook
mapping needs.
- `rela acl map ... --expect <file>`: a CI gate that exits 1 with a diff when
the effective access differs from the file.
- `rela acl audit`: a finding when data-entry.yaml declares webhooks and no
client baseline applies to `webhook`.
- Docs: docs/webhooks.md, docs/acl-overview.md, docs/acl-security.md.

Not in scope:

- The IdP receiver (`POST /webhooks/idp`). It runs one configured action under
a signed JWT; it is a different mechanism and keeps its principal.
- Feed subscriptions (same mapping vocabulary, scheduler principal).
- Per-hook principal types or scopes (see Approach, alternatives).
- Field-level ceiling (`visible:`/`redact:`) in the expectations file.

**Acceptance Criteria:**

1. A webhook delivery runs with `principal_type: webhook`. Test: handler test
reads the principal in a write and asserts the type and user.
2. Without a baseline for `webhook`, a delivery behaves as before. Test: the
existing webhook route tests pass unchanged; a policy test shows the hook keeps
`everyone` + role grants.
3. With `client_baselines: {hooks: {applies_to: [webhook], ...}}` the
delivery is capped: a write outside the baseline is refused (500 from the hook),
a write inside it succeeds. Test: webhook route test with acl.yaml.
4. A foreground automation fired by the webhook save is capped by the same
ceiling. A refused cascade write does not fail the delivery: the hook's own
write stands, the cascade write is refused, and ingest logs the automation
error. Test: route test with an automation that creates an entity of a type
outside the baseline; the hook's entity exists, the cascade entity does not.
5. `rela acl map --webhook <id>` reports the capped access without `--as`.
Unknown hook id is an error. `--webhook` with `--principal/--as/--scope` is an
error. Test: CLI tests.
6. The report lists named permissions held and relation types writable.
Test: aclmap unit test with a policy granting a permission and a relation.
7. `--expect <file>` exits 0 on match, exits 1 with a readable diff on
mismatch, and errors on an unknown type/relation/verb/key in the file. Test: CLI
tests for match, extra grant, missing grant, bad file.
8. `rela acl audit` reports `A15-webhook-uncapped` (medium) when hooks exist
and no baseline covers `webhook`; no finding when one does or no hooks exist.
Test: aclaudit unit test + CLI test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (the approach builds on TKT-IAC8TX; options listed under
Approach).

**Existing Solutions:**

- Client ceiling (TKT-IAC8TX): `acl.Request` compiles the ceiling from
`Principal.PrincipalType()` at `ForPrincipal` (internal/acl/request.go:70).
Every ACL request in the webhook path is built from `principal.From(ctx)`
(visibility/adapters.go, appbuild rebindFor, entitymanager write authz), so
stamping the type on the ctx principal is enough.
- `rela acl map --as/--scope` (internal/cli/acl_map.go,
internal/aclmap/mapprincipal.go) already stamps claims through
`principal.VerifiedFrom` to mirror the request path.
- `acl can-relation` (internal/aclmap/canrelation.go) asks the real gate
`Request.AuthorizeWrite` for a relation write; the report reuses that.
- `acl audit --fail-on` and `acl can` use `errors.NewExitError(1)` for CI.
- Libraries: none needed (yaml.v3 already used).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `internal/principal`: add `TypeWebhook = "webhook"` and
`Webhook(hookID string) Principal`, which returns
`VerifiedFrom(ReservedPrefix+"webhook:"+hookID, ToolWebhookReceiver,
Claims{PrincipalType: TypeWebhook})`. `PrincipalType()` returns `webhook` for
any user named `system:webhook:<id>`, whatever the claims, so a scheduler
`run_as` and every `rela acl` report that builds the principal from its name
meet the same ceiling (design review RR-647UI4).
2. `internal/dataentry/webhook_routes.go`: stamp `principal.Webhook(hookID)`
and clear any ctx `acl.Request` after the swap. `internal/ingest` logs
`AutomationErrors` after a write. 2a. `internal/acl`:
`Request.ClampedEveryoneGrants` and `Request.ClampedAssertedGrants` apply the
ceiling to the everyone and asserted routes the map reports (RR-PYELJT). The map
adds per-face grants (`TypeAccess.Faces`) so a `type@face` grant is visible
(RR-YMQ2B9).
3. `internal/aclmap`: add `MapWebhook(ctx, hookID, verb, type, entityTypes,
Extras)` and `MapPrincipalFull(..., view, Extras)`; `MapPrincipalAs` delegates
with no extras. `Extras` carries the permission names and relation types to
probe. The result gains `Webhook`, `Permissions` and `Relations` (omitempty,
additive JSON). Permissions use `Request.HoldsPermission`; relation writes use
`Request.AuthorizeWrite` with a `RelationSubject` per (relation, from type, op),
with `FamilyFaces` from the schema, as can-relation does.
4. `internal/cli/acl_map.go`: `--webhook <id>` (exclusive with
`--principal/--as/--scope`), validated against data-entry.yaml. The permission
universe is the union of the roles' permission lists ("*" matches no named
permission, so nothing else can be held). The relation universe is every
relation type in the schema. Text output adds "permissions" and "relation
writes" sections and, for `--webhook`, a check of what the hook mapping needs:
read on the find type (at `find.face`), create, update, and `find.match`
properties the field ceiling hides.
5. `internal/cli/acl_expect.go`: `--expect <file>` loads a strict YAML file
(`types`, `permissions`, `relations`), validates names against the schema,
computes the full report (requires `--principal` or `--webhook`, refuses
`--verb`/`--type`), and prints a sorted diff (`+` held but not expected, with
the granting routes; `-` expected but not held). `verb` means type-wide,
`verb:some` means on some entities only, `type@face` keys hold face-only grants,
`rename` is rejected. Exit 1 on any difference or a failing webhook check; JSON
adds an `expectation` object.
6. `internal/aclaudit`: `CheckWebhooks(p, hookIDs)` returns
`A15-webhook-uncapped` (Medium) when hookIDs is non-empty and no baseline lists
`webhook` in `applies_to`. The CLI passes the hook ids from data-entry.yaml.
7. Docs as listed.

Per-hook vs single type: a single type `webhook` is enough. The ceiling answers
"what may any webhook do at most"; the per-hook distinction already exists in
the identity (`system:webhook:<id>`), which acl.yaml assigns a role per hook. A
per-hook ceiling would duplicate that knob, and scopes are an IdP vocabulary
that re-open capability rather than narrow it. A `webhook:<id>` scope can be
added later without breaking anything, if a deployment needs a narrower baseline
with per-hook re-openings.

Alternatives rejected:

- A new `rela acl webhook` command: duplicates `acl map` output and flags.
- Only an audit finding for "rights beyond the mapping": a permanent finding
that cannot be suppressed when two hooks share a role on purpose (Atlas's
recovery hook). The map shows it as information; `--expect` gates it.
- A per-hook principal type (`webhook:<id>`): `applies_to` sets must be
disjoint and would need one baseline per hook.

**Files to modify:**

- internal/principal/principal.go (+ test)
- internal/dataentry/webhook_routes.go (+ webhook_routes_test.go)
- internal/aclmap/mapprincipal.go, new internal/aclmap/capabilities.go (+ tests)
- internal/cli/acl_map.go, new internal/cli/acl_expect.go, internal/cli/acl.go (+ tests)
- internal/aclaudit/ceiling.go (+ test)
- docs/webhooks.md, docs/acl-overview.md, docs/acl-security.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Hook id: from data-entry.yaml (operator config), already validated; the
route only exists for configured ids.
- `--webhook` value: checked against the configured hooks; unknown id errors.
- Expectations file: operator-authored YAML, decoded with KnownFields; type,
relation and verb names checked against the schema and verb allowlists.

**Security-Sensitive Operations:**

- The principal type only narrows: a type no baseline matches is
unrestricted, so deployments without a `webhook` baseline see no change.
- The claim is set through `VerifiedFrom`; no request input reaches it. The
name-derived type only narrows, and `system:` names cannot be asserted through a
principal header.
- Not capped: `background: true` automation actions (they re-stamp
`system:automation` or `run_as`) and `allow_acl_bypass` scripts. Documented.
- Follow-up TKT-RQ9HLJ: the IdP receiver identity `webhook:<event>` is not
under the reserved prefix (pre-existing).
- The `system:` reserved prefix is unchanged, so `-principal-header` still
cannot assert a hook identity.
- `principal_type` now appears in audit records of webhook writes (additive,
omitempty field).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: route test captures the principal of the write (audit or a recording
writer) and asserts user + `PrincipalType() == "webhook"`.
2. AC2: existing route tests; acl test that a `webhook` principal without a
matching baseline keeps everyone grants.
3. AC3/AC4: route test with acl.yaml + baseline + automation (integration
through the real router and manager).
4. AC5-AC7: CLI tests in internal/cli with a temp project (the acl_map_as
tests' fixture style).
5. AC8: aclaudit unit + CLI audit test.

**Edge Cases:**

- Hook with only `create_if_missing` (no find): needs create only (+ update
when `then` steps exist).
- Hook with find only: needs read (+ update when `then` steps exist).
- Policy without client_baselines: map shows uncapped access; audit A15.
- No acl.yaml: map prints the existing "no policy" line; `--expect` errors
(cannot attest under no policy) rather than passing.
- Expectations file empty: expects no access at all.
- Per-entity exceptions are `verb:some` in the expectation, so widening a
grant from some entities to the type is a difference.
- Hook whose find match property the ceiling hides: the check reports it and
`--expect` exits 1.

**Negative Tests:**

- `--webhook` unknown id → error, exit 1.
- `--webhook` with `--principal` / `--as` / `--scope` → error.
- `--expect` with `--verb` or `--type` → error.
- `--expect` file: unknown key, unknown type, unknown relation, bad verb →
error naming the entry.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- A deployment whose IdP already issues `principal_type: webhook` for tokens
and has a baseline for it would now also cap declarative hooks. Mitigation:
document; the effect is narrowing only.
- A webhook-triggered automation that needs rights outside the baseline
starts failing once an operator adds a `webhook` baseline. Mitigation: document;
`acl map --webhook` shows the capped rights before deploying.
- Relation report on faced types: approximation of the write gate by
family faces. Mitigation: same subject construction as can-relation.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- docs/webhooks.md: ACL section (principal type, ceiling, automations, CLI).
- docs/acl-overview.md: webhooks under "Restricting a client below its user".
- docs/acl-security.md: A15 in the audit list; `acl map --webhook`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the review-response entities linked to the
ticket.
