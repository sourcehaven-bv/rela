---
id: TKT-IAC8TX
type: ticket
title: 'Client attenuation: principal_type baselines + scope re-openings as an ACL ceiling below the acting user'
kind: enhancement
priority: medium
effort: l
status: done
---

Let a verified non-interactive client (MCP, app, PAT, service account) be
restricted **below** the user it acts as. Named `client_baselines` are selected
by a disjoint `principal_type` match; `scope_grants` re-open capability on top.

```
effective = user_grants ∩ (baseline ∪ scope_grants)
```

Compiled at load into plain allowlists, so the runtime evaluator is unchanged —
no denial primitive, DEC-RG878 union semantics intact.

## Problem

An OIDC proxy (Pratique) mints principals carrying `principal_type`
(`user`/`app`/`pat`/`service`), `scope` and `client_id`. rela reads **none** of
them — `internal/jwtauth.VerifySubject` projects only `sub`, `email`, `org_id`,
`org_slug`, `roles` (`verifier.go:157-163,186-204`), and `email` is dropped
again at the `dataentry.AssertedIdentity` adapter (`router.go:471-476`).

Consequence: a client acts with the **full authority of the user it acts as**.
An MCP server connected by an HR user can read `person.salary` because that user
can. There is no way to say "this client, acting as this user, sees less."

TKT-RP3X3Q explicitly deferred this: *"`principal_type` — Pratique's own
middleware does not surface it; if rela needs to distinguish a PAT from an
interactive user that is its own ticket."* This is that ticket.

Aggravating factor: **MCP reads are entirely ungated today.** `internal/mcp`
imports neither `internal/acl` nor `internal/visibility`; handlers call the raw
store (`tools_entity.go:35,86,124,201,246,297`). Writes are gated downstream by
entitymanager, reads are not. Documented at
`docs/server-security.md:340,389-396` and `docs/acl-security.md:743-746`,
tracked as TKT-G3PPD.

## Design

### The invariant

Two properties fall out of `user_grants ∩ (baseline ∪ scope_grants)`, and both
are load-bearing:

1. **A ceiling never grants.** Intersection with `user_grants` means a token
cannot exceed the user it acts as, whatever it claims. A read-only user with a
write-scoped token still cannot write.
2. **Scopes widen only within the ceiling.** `baseline ∪ scope_grants` is a
union, so more scopes = more capability — matching how OAuth scopes work
everywhere else — but still clamped by (1).

### Why this shape, and not the alternatives

Considered and rejected in design discussion:

- **Explicit deny rules subtracting from the union at runtime.** Would invert
DEC-RG878's additive semantics and force re-derivation of `authorizeWrite`,
`readQuery` (which compiles to `store.GraphQuery` and pushes into SQL),
`PermitsReadMany`, `FieldVerdicts`, `TransitionVerdicts`, `AccessRoutes` and all
of `internal/aclmap`. **Note the correction:** a deny rule applied at *load*
time — subtracting from the role's allowlists to produce a derived narrowed
allowlist — needs none of that. That realisation is what this design rests on.
The runtime never learns the word "deny"; it still sees a plain allowlist.
Wildcards expand against the metamodel at load (`ValidateAgainstMetamodel`
already runs there).
- **Intersecting denial sets across profiles.** `{salary} ∩ {bsn} = {}` — the
denials cancel, and every profile must restate every denial or it evaporates.
Rejected: the restatement cost is the thing we are trying to avoid.
- **A flat per-profile allowlist that restates everything.** Clear as glass, but
effort is O(schema size) per profile and drifts as the metamodel grows.

The baseline+scopes shape puts the effort where it belongs: the baseline is
written once per client class, and a scope's cost is proportional to what it
re-opens.

### Selection: exactly one baseline, no combination rule

`client_baselines` are named blocks, each declaring `applies_to:
[<principal_type>…]`. **The `applies_to` sets must be disjoint — overlap is a
load error, not a precedence puzzle.** So exactly one baseline matches any
token, and there is no baseline-vs-baseline combination semantics to specify or
document.

A `principal_type` matching no baseline is **unrestricted** (gated only by the
user's own roles). This is what makes `principal_type: user` work with no
special-casing. It also means an unknown type string from another provider
escapes the ceiling — hence the audit rule below.

`Tool` was considered as a selector and **dropped**: it is self-asserted by the
entry-point binary, not cryptographically verified, and mixing a spoofable key
with signed claims in one mechanism invites operators to lean on the weak one.
stdio MCP (which has no authentication at all) is therefore not addressable by
this feature; that is TKT-G3PPD's problem, not this one.

### Allowlist or denylist, per type, per block

Each block picks the posture that fits, per entity type:

| Allowlist (fail-closed) | Denylist (low-effort) |
|---|---|
| `visible: {person: [name, email]}` | `redact: {person: [salary, bsn]}` |
| `read: [ticket, person]` | `deny_read: [audit-record]` |
| `update: [ticket]` | `deny_update: ["*"]` |

Declaring both spellings for the same type in one block is a **load error**.

`deny_write: ["*"]` is shorthand expanding at load to create+update+delete —
"read-only client" should be one line, not three.

**Omitted axis = inherited** (the block does not narrow it).

The allowlist form inherits the existing **closed-world `visible:`** semantics
(`affordances/resolver.go:370` — a role declaring a `visible:` block for a type
asserts a complete list; unnamed fields redact). That is what gives the
fail-closed property: a field added to `person` tomorrow is hidden from any
client whose block names `person` under `visible:`, with no operator action.

### Verbs vs permissions stay separate

`RoleDef` has two distinct axes and this ticket mirrors both rather than
unifying them:

- **verb × entity type** — `Create`/`Update`/`Delete`/`Read []string`, evaluated
by `grantsVerb(role, op, target)` (`policy.go:288`).
- **named capabilities** — `Permissions []string`, evaluated by
`grantsPermission` (`resolver.go:272`), flat string match, no type parameter.

So `update: [ticket]` is not expressible as a permission (permissions have
nowhere to put the type). `deny_permissions:` covers the second axis and reaches
every `permission:` value in the system — command guards, document guards,
navigation, `history:read`, `history:read-redacted`, delegate-X.

**IDEA-HUWQ** proposes collapsing these two axes into one named-permission
catalog, and its own sequencing note says to hold until TKT-VMD8 lands. Unifying
here would mean redesigning the core grant model inside a client-restriction
ticket. If IDEA-HUWQ later lands, this ceiling collapses with it for free — both
spellings already compile to the same clamp at load.

## Worked example

Metamodel: `person` (name, email, phone, salary, bsn), `ticket` (title, status,
body, assignee), `audit-record` (actor, action, timestamp).

```yaml
roles:
  hr:
    read:   [person, ticket, audit-record]
    update: [person, ticket]
    delete: [ticket]
    permissions: [history:read]
    # no visible: block -> Alice sees ALL fields

client_baselines:
  apps:
    applies_to: [app]
    deny_write: ["*"]
    deny_permissions: [history:read]
    redact:
      person: [salary, bsn]
    # read omitted -> inherits the user's rows

  service-accounts:
    applies_to: [service]
    read: [ticket]
    visible:
      ticket: [title, status]

scope_grants:
  rela.people.read:
    read: [person]
    visible: {person: [name, email]}
  rela.tickets.write:
    update: [ticket]
```

Alice (role `hr`) connects an MCP client; token has `principal_type: app`,
`scope: "rela.tickets.write"`.

- **read** — baseline omits it, `rela.people.read` absent → inherits Alice's
`{person, ticket, audit-record}`.
- **person fields** — `redact:` removes salary and bsn. Alice still sees them in
the SPA; the MCP does not. **This is the original ask.**
- **update** — `deny_write: ["*"]` ∪ `rela.tickets.write` → `{ticket}`,
intersected with Alice's `{person, ticket}` → `{ticket}`. If read-only Bob used
the same token: `{} ∩ {ticket} = {}`. The ceiling never grants past the user.
- **history:read** — withheld despite Alice's role granting it.

A `service` token with no scope sees ticket rows only, title+status only —
person rows 404 (row-level, not redaction: nonexistence is the secret, per the
row-level rule in CLAUDE.md).

## Scope

**In scope**

- `principal_type` + `scope` claim extraction: `jwtauth.AssertionClaims`
(`verifier.go:157-163`) → `dataentry.AssertedIdentity` (`router.go:471-476`) →
`principal.Verified` (`principal.go:84`). Bounded like `roles` is
(`maxRoles=32`, `maxRoleRunes=256`, `verifier.go:171-174`).
- New `Policy` fields + `knownPolicyKeys` (`policy.go:413-424`, guarded by the
reflection parity test `policy_parity_test.go:15-30`).
- Load-time compilation of baselines/scope_grants into plain allowlists,
including `"*"` expansion against the metamodel.
- The clamp, applied after role resolution: `computeGlobals`/`ForEntity` results
narrowed before `decideFromAttrs`, `readQuery`, `grantsPermission`,
`FieldVerdicts`.
- Enforcement on every surface the ACL already reaches: read gate, `visible:`
redaction, write authorize, named permissions.
- A distinguishable `SourceKind` so attribution names the ceiling that clamped.
- `rela acl audit` rules (below) + `rela acl map --as <profile>` to render a
compiled ceiling.
- Docs: `docs/acl-security.md` (trust-boundary section), `docs/acl-overview.md`
(pinned by `docfields_test.go` / `docs_example_test.go`).

**Out of scope**

- **MCP `list_tools` filtering** → TKT-G3PPD. A client denied writes still sees
write tools and gets a runtime rejection.
- **MCP read gating generally** → TKT-G3PPD. `internal/mcp` bypasses the read
path entirely; this ticket makes the *policy* expressible, it does not wire MCP
into `internal/visibility`.
- **Org enforcement** (`OrgID`/`OrgSlug` are attribution-only,
`principal.go:100-104`) — separate ticket, named in TKT-RP3X3Q.
- **`client_id` keying** — the mechanism supports adding a third table later at
no semantic cost (still exactly-one-baseline); not built now.
- **Unifying verbs and permissions** → IDEA-HUWQ.
- **`syncContext` claim-dropping** (below) — pre-existing, its own ticket.

## Acceptance criteria

1. A verified assertion carrying `principal_type` and `scope` yields a
Principal exposing both; absent claims yield empty values with no error
(`--principal-header` and loopback paths unchanged).
2. Two `client_baselines` whose `applies_to` sets overlap is a **startup error**
naming both blocks.
3. A token whose `principal_type` matches no baseline is unrestricted —
pinned by an explicit test so it cannot drift.
4. `redact:` hides exactly the named fields; a field added to the type later
still shows. `visible:` hides everything unnamed, including fields added later.
Both pinned.
5. Declaring `visible:` and `redact:` for the same type in one block is a load
error.
6. `deny_write: ["*"]` denies create, update and delete.
7. A scope re-opens capability the baseline closed, bounded by the user: the
same token used by a lesser-privileged user grants strictly less.
8. `deny_permissions:` withholds a named permission the user's role grants
(verified against `history:read` and a command `permission:`).
9. Row-denial is indistinguishable from nonexistence — 404, pruned lists, no
count leak (per the row-level rule).
10. `rela acl map --as <profile>` renders the compiled effective ceiling.
11. `rela acl audit` flags: a `principal_type` value no baseline covers; a
baseline that narrows nothing (dead config); a block naming an undeclared
type/field.
12. Attribution names the ceiling in the deny reason, distinguishably from a
role-grant denial.
13. `NopACL` / `ReadOnlyACL` unchanged.

## Risks

- **Trust boundary.** `internal/acl` verifies nothing; a Principal is trusted
absolutely. `principal_type`/`scope` MUST be populated only after signature
verification — the unexported-field + `Verified()` constructor shape
(`principal.go:58-95`) already enforces this at compile time and must be
preserved for the new fields. A spoofable-header path setting them would be a
full bypass.
- **A ceiling that silently grants nothing.** An operator writing a baseline for
a `principal_type` the IdP never sends gets no error and no protection — AC11's
audit rule is the mitigation.
- **`syncContext` (`sync_handlers.go:20-23`) already drops all verified claims
today** — a plain composite literal, exactly the failure
`resolvePrincipalEntity` (`router.go:380-382`) warns about. New fields would be
dropped too. Pre-existing bug; **file separately** rather than fixing inline.
- **Eleven plain-composite-literal Principal construction sites** (CLI, MCP,
scheduler, desktop, docs, webhook, aclmap ×2, docscapture, header/env resolvers)
need a zero-value decision for the new fields.
- **`rela acl map` cannot model asserted claims today** (`mapall.go:71-74`) —
AC10 requires extending it.
- **Adding a field to `Principal` touches nine methods** (`Verified`,
`Sanitized`, `Equal`, `IsZero`, `Clone`, `principalJSON`, `MarshalJSON`,
`UnmarshalJSON`) plus the audit wire format.
- **9-minute stale-claim window** — assertion TTL is hard-coded upstream
(`signer.go:22`); a scope change takes up to 9 minutes to take effect. Document,
do not try to fix.

## Prior art

- Extends **FEAT-AESD4**, whose design already calls for exactly this:
*"MCP transport intersects user capabilities with agent scope and defaults
agents to read-only."* This is the policy half of that.
- **DEC-RG878** — union semantics preserved (load-time compilation).
- **TKT-RP3X3Q** — deferred `principal_type` here; established
`asserted_role_assignments` as the precedent for claim→policy mapping and the
`SourceAsserted` attribution kind.
- **TKT-G3PPD** — MCP transport intersection, consumes this policy.
- **IDEA-HUWQ** — would later collapse the verb/permission duality.
