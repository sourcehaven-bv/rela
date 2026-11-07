---
id: PLAN-FIKLS3W
type: planning-checklist
title: 'Planning: Store IdP-asserted roles per principal for background jobs (R14)'
started: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: storing IdP role names per subject from the verified webhook path; applying
them to scheduler `for_each` children; `rela.principals_with_role(role)`.

Out: stored roles on CLI, stdio MCP, header-principal and JWT requests (a JWT
carries its own, fresher roles); org enforcement (R1); `rela acl audit`
coverage (TKT-IKCDFM).

**Acceptance Criteria:**

1. A webhook action returning `asserted_roles = {"cpo"}` stores `cpo` for the
   verified `user_id`; `asserted_roles = {}` removes the record.
2. The HTTP action endpoint refuses a response carrying `asserted_roles`.
3. A `for_each` child for a user entity whose subject has stored `cpo`, mapped
   to role `officer`, holds `officer`.
4. `rela.principals_with_role("officer")` returns that user entity, and omits
   it when the script cannot read it.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approach follows the leases/piles state precedent; options are recorded below)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal state seam, no external analogue needed)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `internal/lease/kvlease`: one KV document, read-modify-write under a lock.
- `internal/lock.For(store)`: in-process lock, or a schema-scoped postgres
  advisory lock.
- `internal/appbuild/realtime_outputs.go` `asRetry`: rebuilds a principal with
  roles verified earlier, through `principal.VerifiedFrom`, in a background job.
- `aclstore.PrincipalLookup`: indexed `principal_property` lookup.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`internal/idproles.Store` keeps `{subject: {org_id, roles, updated_at}}` in one
`state.KV` document, written under `lock.Locker`. The webhook receiver stores
the `asserted_roles` the action returns for the verified subject. The
scheduler's `for_each` provider returns a principal built with
`principal.VerifiedFrom` carrying the stored roles. `rela.principals_with_role`
maps the ACL role to IdP claims through `asserted_role_assignments`, the claims
to subjects, the subjects to user entities, and filters them through the
script's gated reader.

Alternatives rejected: a system-managed property on the user entity (writable
through the entity write path, so self-promotion); a dedicated postgres table
(a second backend and migration for data the state KV and advisory lock already
serve); one KV key per subject (no way to enumerate holders).

**Files to modify:**

`internal/idproles/` (new), `internal/script/action.go`,
`internal/dataentry/webhook.go`, `internal/dataentry/actions_ops.go`,
`internal/scheduler/foreach.go`, `internal/appbuild/scheduler_foreach.go`,
`internal/appbuild/idproles.go` (new), `internal/lua/roleholders.go` (new),
`internal/lua/deps.go`, `cmd/rela-server/serve.go`, docs.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Subject and org: from the verified webhook JWT only, never from the script.
- Role names: from the action's return value (fetched from the IdP operator
  API). Trimmed; empty, over-long or control-character names and more than 64
  roles are refused, so the webhook answers 502 and the IdP retries.
- Role argument of `rela.principals_with_role`: must name a declared ACL role.

**Security-Sensitive Operations:**

- Storing roles: only the webhook receiver writes, after signature checking.
- Applying roles: only the `for_each` child path; roles still map through the
  operator-authored `asserted_role_assignments`, so a claim cannot name an
  unmapped role. The subject must map back to exactly the selected entity.
- Listing holders: results are filtered through the caller's gated reader.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. `idproles` table tests: set, replace, clear, list by claim, validation.
2. Webhook handler test with a stub store: stored on success, not stored on
   failure or `message_type = error`.
3. HTTP action returning `asserted_roles` fails.
4. appbuild test: for_each principal carries stored roles; ACL grants the
   mapped role.
5. Lua binding test with a fake holder source and gated reader.

**Edge Cases:**

- No principal_property: the subject is the entity id.
- Ambiguous or mismatched subject mapping: no roles.
- Damaged document: error, no roles applied.
- Concurrent writes: serialized by the lock.

**Negative Tests:**

- Invalid role names, too many roles, unknown ACL role in the Lua call.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Stale roles if a revoking webhook is lost: documented; the next webhook for
  the subject corrects it.
- Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- idp-webhook-provisioning, acl-security, scheduled-tasks and lua-scripting
  guides in `docs-project/`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the requester asked for a rela-security-reviewer pass on the diff instead)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above; security review findings are tracked as review responses)

**Design Review Findings:** N/A
