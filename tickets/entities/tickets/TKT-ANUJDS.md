---
id: TKT-ANUJDS
type: ticket
title: 'Provision a stub user entity for an unmatched verified principal (unmatched_principal: provision)'
kind: enhancement
priority: medium
effort: m
status: review
---

Follow-up to TKT-0C3II2, which shipped `unmatched_principal: anonymous | reject`
and left `provision` as a **reserved** value (accepted at load; behaves as
anonymous + a one-time warn). This ticket implements `provision`: lazily create
a minimal stub user entity for a verified JWT principal whose subject resolves
to no `user_entity_type` entity, so the principal gains a graph identity.

The full design was worked out and parked while `reject` shipped. This ticket
reconstitutes it from `.ignored/provision-unmatched-principal-design.md`. **The
write-seam is NOT yet resolved** — see the open problem below; that is the first
thing planning must settle.

## The flow

user → Pratique (auth) → rela receives the verified principal →
principal_property lookup: exists → normal path → absent  → provision a stub
(this ticket) → on the create, the operator's on-create AUTOMATIONS may run

## Decided design (do not re-litigate — settled with the user)

1. **Declarative, not Lua.** rela creates the stub in built-in Go from the
verified assertion claims. It does NOT run the config's Lua provisioning action
(that is the IdP webhook's job — `webhook.go dispatchWebhookAction`, which
enriches from the IdP over HTTP). The read-path rule forbids *user-supplied* Lua
on reads, not a fixed built-in write.

2. **Lazy on first WRITE, never on a GET.** A GET by an unmatched principal
stays read-only (anonymous, or 403 under `reject` — already shipped).
Provisioning fires inside the first authorized write, via
`entitymanager.Manager` under a `system:provisioner` principal — a
write-triggered write, which the architecture permits.

3. **BARE STUB ONLY — no group membership at provision time** (resolves the
deferred critical, below). `system:provisioner` stays truly minimal: `create:
[user_entity_type]` and nothing else. rela does NOT rely on the on-create
cascade to add a `member-of` edge — the cascade authorizes as the provisioner,
which lacks relation-create rights. The stub is NOT inert: **asserted JWT roles
already apply** independently of the graph entity (TKT-RP3X3Q).
Groups/local-roles arrive later, out of scope: the webhook, a reconcile, or an
admin. An operator on-create automation that only sets *properties* still runs;
one needing extra write grants fails — now correct/expected, documented as
operator responsibility.

4. **Stub properties.** `sub` → the declared `principal_property` (join key;
already required `unique:true`). Plus `email` (thread it through
`dataentry.AssertedIdentity` + `verifiedPrincipal` + `principal.Principal` — the
assertion carries it, `AssertedIdentity` currently drops it), `org_id`,
`org_slug` as plain properties, only if the metamodel's user type declares them
(else the create soft-warns and drops them). No org *relation*.

5. **System principal.** `principal.UserProvisioner = "system:provisioner"` +
`ToolProvisioner = "provisioner"`. A migration mirroring
`migration/acl_scheduler_grant.go` injects `create: [<user_entity_type>]` and
binds the principal — only when acl.yaml exists AND `unmatched_principal:
provision` AND `user_entity_type` is set. Principal hardcoded as a literal
(arch-lint bars migration→principal import), with a lockstep equality test.

6. **Run the create under the provisioner** (`principal.With(ctx, provisioner)`
before `CreateEntity`) so it's authorized against and audited to
`system:provisioner`.

## THE UNRESOLVED PROBLEM — the write seam (RR-BZQ049 provision-half + RR-9XBIJZ)

`reject` was enforceable at the single write-authorization point
(`Declarative.AuthorizeWrite`) because it is a pure gate — no write, no
`writeMu`, no re-stamp. **`provision` cannot reuse that seam** because it must
perform a write and re-stamp the ctx mid-request.

- A per-CRUD-handler hook misses sync, Lua-action, and attachment writes (the
bypass RR-BZQ049 found). Attachments and sync go through the write handlers;
git-sync is out of surface (not an entity write).
- Hooking in `attachACLRequest` (the read middleware) is wrong: no
`writeMu`/manager there, and holding `writeMu` would deadlock the downstream
handler that also takes it.
- **RR-9XBIJZ:** the re-stamped ctx must actually reach the manager call
(callers must adopt it, not discard it), AND the read gate — a separate memoized
`acl.Request` built at `attachACLRequest` time on the pre-provision principal —
must be rebuilt on the new ctx, or the response-shaping read runs as the
anonymous principal and can redact/404 the just-created entity out of its own
response.

Candidate directions (evaluate in planning): (a) a write-only middleware layer
wrapping the mutating routes that acquires `writeMu`, provisions, rebuilds the
Request+gate, and passes the new ctx down; (b) find/build a single
post-`writeMu` choke point every mutation funnels through. The likely answer is
a real analysis of the handler/lock structure — that is the design pass this
ticket needs before code.

## Other open items

- **Idempotency / races.** Two concurrent first-writes from one sub must create
exactly one entity. `writeMu` serialises in-process; `principal_property` is
`unique:true` but NOT atomically enforced at write time on fs/mem
(`checkUniqueProperties` is check-then-write). Tolerate `store.ErrConflict` +
re-resolve for cross-process; a pgstore partial unique index closes it fully.
- **Webhook duplicate.** The IdP webhook provisions the SAME person entity from
a membership event, also under `writeMu`. On fs/mem, webhook + lazy-provision
(or two processes) can create two stubs with the same sub → `ResolvePrincipal`
becomes permanently ambiguous → all future grants for that sub lost. Reconcile
the two paths through one shared idempotent helper, or document the fs-backend
limitation.
- **Email field growth.** Adding `email` to `principal.Principal` touches the
struct, `Verified` (already 5 params — consider a claims struct), `Sanitized`,
`Equal`, `principalJSON`/`Marshal`/`Unmarshal` (audit wire, omitempty), the
accessor. Same unexported-field treatment as org/roles.

## Acceptance criteria (draft — finalise in planning)

1. `provision` ⇒ the first authorized write by an unmatched verified principal
creates the stub (keyed on `principal_property = sub`, with org + email) under
`system:provisioner`, then the write proceeds; a GET does not provision.
2. The triggering write sees its own newly-provisioned entity (re-stamp reaches
the manager; read gate rebuilt) — no one-request-behind.
3. Concurrent first-writes from one sub create exactly one entity.
4. `provision` without the system principal / grant wired fails at LOAD.
5. The stub create is audited to `system:provisioner`.
6. Provisioning covers every entity-write path (CRUD, sync, action, attachment),
not just CRUD — the anti-bypass invariant, pinned like the reject test.

## Prior art (all file:line in the parked doc)

- `principal.UserScheduler` + `migration/acl_scheduler_grant.go` — the system-
principal + migrated-grant pattern.
- `webhook.go dispatchWebhookAction` — provisioning write under a dedicated
principal, under `writeMu`; the other path that creates the same person.
- `entitymanager` create → `Cascade.Process` (synchronous) — the automation
mechanism.
- `Declarative.AuthorizeWrite` opening a FRESH Request from `principal.From(ctx)`
with a live graph walk — why a re-stamp lets the triggering write see the new
entity.
- `unmatched_principal` key (TKT-0C3II2) — the policy key this extends from
reserved to implemented.
