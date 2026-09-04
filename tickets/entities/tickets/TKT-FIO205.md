---
id: TKT-FIO205
type: ticket
title: 'Entity commenting stage 1: property and section anchors'
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Stage 1 of entity commenting (FEAT-002WMX), per the anchoring research in
RES-XRYX18. Design-reviewed 2026-09-01 (8 findings, all addressed below).

Users can attach a comment to a **named property** or a **view section** of an
entity, and read the resulting thread. These two anchor kinds are deliberately
drift-free: a property name and an operator-authored `sectionId` are *names*,
not offsets, so they survive any edit to the entity body.

**Comments are not part of the graph.** A comment is a remark *about* an entity,
not a fact *in* the operator's domain model. So comments are their own thing:
their own service, their own storage interface with real backends, and their own
ACL permissions — outside `store.Store` and `entitymanager`. rela owns the read
and write path end to end, which is what makes `author` unforgeable: the handler
writes it from `principal.From(ctx)` and never reads it from the request body.

Text-range anchoring via `github.com/vloothuis/textanchor` and `FormatMarkdown`
normalisation follow in stage 2 and are **out of scope here** — but the anchor
field layout must not foreclose them.

## Scope

### Storage — a real interface with real backends

`internal/comments` defines the `Comment` record and a `comments.Store`
interface (`List`, `Add`, `Update`, `Delete`), plus a conformance suite
(`commentstest.RunAll`) every backend must pass — the `store.Store` /
`internal/store/storetest` pattern, not a KV blob. Comments are records to be
listed and permissioned per target; a blob forces read-modify-write on every add
and makes per-entity queries impossible.

**This ticket ships two backends** (RR-JT560T): `filecomments` (default,
YAML/TOML under `.rela/comments/`) and `memcomments` (tests). The interface is
designed for four; `pgcomments` and `sqlitecomments` are a follow-up ticket that
must need **no interface change** — the conformance suite is what makes that
cheap.

**Contract the interface must state explicitly** (RR-7F6NM9, RR-3VSSPM):

- The **server mints the comment ID**; a client-supplied ID is ignored. Same
rationale as author stamping — otherwise a caller can overwrite another user's
comment by reusing an ID.
- `List` returns comments ordered by `CreatedAt` ascending, with the minted ID
as a deterministic tie-break. Pinned by `commentstest.RunAll` so backends cannot
disagree.
- Writes to one target are **serialised** (the `store.Store.Tx` role, DEC-8UIL0).
Two concurrent adds must both survive; conformance-tested.
- The file backend writes **atomically** (temp file + rename). One file holds N
comments, so a truncated write loses a whole thread, not one record.

### ACL — six named permissions, resolved per target entity

`comment:read`, `comment:add`, `comment:update-own`, `comment:update-any`,
`comment:delete-own`, `comment:delete-any`.

Granted through a role's `permissions:` list like the existing `history:read`
family, registered in `acl.BuiltinPermissions()` (`policy.go:72`) — a guard test
(`permguard_test.go`) fails otherwise. Naming follows the
`history:read-redacted` hyphenated-qualifier convention.

Resolution uses **`acl.Request.HoldsPermissionForEntity`** (`resolver.go:260`),
so a permission conferred by an ownership relation *to the target entity* is
honoured, not just a global grant — the seam the statemachine transition guard
already uses.

**Why own/any are explicit permissions rather than the graph's ownership
mechanism** (RR-3VSSPM): rela has no `own` primitive. Ownership is a graph edge
(`role_relations: {assigned-to: {confers: assignee}}`) tested by `graph.HasEdge`
(`resolver.go:176`). Comments are not in the graph, so there is no edge to test;
reusing that machinery would mean teaching the ACL graph walker about a
non-graph store — the exact coupling that separating comments avoided. "Own" is
instead a string comparison of the stored `Author` against
`principal.From(ctx)`, inside the comment service, needing no ACL change. `-any`
implies `-own`.

**Two invariants:**

- `comment:read` is **floored by the target's read verdict**. A principal who
cannot read the entity cannot read its comments however comment grants read, and
cannot distinguish "none" from "denied".
- **Mutating permissions require `comment:read`** (RR-60067I), validated at load
exactly as `policy.go:1053` rejects entity `update`/`delete` without a covering
`read`. `comment:add` is **exempt**, mirroring the `create` exemption
(TKT-4LQMWP): write-only commenting is a legitimate posture.

### Input validation (RR-OOPBUZ)

Enforced at the handler, the trust boundary, allowlist-shaped:

- Body: non-empty, max length (bounded like `cmdexec`'s output cap); control
characters rejected except `\n`/`\t` — the file backend serialises to YAML/TOML
where a NUL is lossy.
- Per-target comment cap, to bound both file size and list cost.
- Target `{type}`/`{id}` path segments validated with `isSafePathSegment`
(`middleware_security.go:568`) before reaching the file backend.
- `anchor_ref`: a safe identifier; one that matches nothing is a **warning, not
a 422** (DEC-HWZHA).

### Counts are post-gate (RR-1PCQ42)

Any comment count (per-property badge, panel header) is computed **after** the
`comment:read` verdict, never from a raw store count —
`docs/acl-security.md:638` ("no count from an unfiltered source"). If a cap is
applied, `truncated` is computed post-filter, following
`TestGantt_TruncatedIsPostFilter`.

### Lifecycle (RR-FCUS1V)

The comment store subscribes as a **`store.EntityObserver`**:

- `EntityRenamed(oldID, renamed)` → **re-key** the target's comments. Rename
emits *only* this callback, never delete+put (`store.go:996`), and the godoc
names ID-keyed reference stores as the intended consumer. Without this every
comment on a renamed entity is silently unreachable.
- `EntityDelete(id)` → remove that target's comments.

### Also in scope

- Fixed, server-written fields: `ID`, `Author`, `CreatedAt`, `Anchor{Kind,Ref}`,
`Body`, `Resolved`.
- HTTP surface under `/api/v1/_comments/...`.
- Creating, resolving and deleting a comment from the SPA, anchored to a
property or a section.
- A `comments:` metamodel block carrying policy only (enable + commentable
types). Declares no schema, synthesises no types.

## Out of scope

Later increments: `pgcomments` / `sqlitecomments` (follow-up ticket);
operator-declared extra comment fields; text-range anchors + `FormatMarkdown`
normalisation (stage 2); content-hash staleness pinning (stage 3); threading.

Decided against, permanently: **no entity type** (comments never enter
`store.Store`, `entitymanager`, the audit log or `/_schema`); **no versioning**;
**no search indexing**.

## Acceptance criteria

1. With no `comments:` block: `/_comments/` routes 404, and no comment storage
file is created (RR-17JRWP — restated from "byte-identical").
2. With the block enabled for a type, a comment can be created against one of
its properties and read back.
3. A create request supplying `author` **or** `id` in the body has both ignored;
the server's principal and minted ID win.
4. A comment against a type not listed as commentable is refused.
5. Each permission is independently enforced: `read` without `add` lists but
cannot create; `update-own` cannot edit another author's comment; `update-any`
can.
6. A permission conferred by an ownership relation to the *target* is honoured.
7. A principal who cannot read the target cannot read its comments, whatever
their comment grants, and gets an indistinguishable 404.
8. A policy granting a mutating comment permission without `comment:read` fails
at load.
9. A detached `anchor_ref` still renders, flagged — warning, never a 422.
10. Rename re-keys comments; delete removes them.
11. Two concurrent adds to one target both survive.
12. Body exceeding the cap, or containing control characters, is refused.
13. Comment counts are post-gate.
14. Both backends pass `commentstest.RunAll`.

## Notes

- Author cannot be stamped from within the graph machinery, which is why rela
owning the write path matters: `{{user.name}}` is the git config user
(`automation/template.go:48`); `computed:` cannot see the principal
(`computed/computed.go:117-121`); `admin.update_entity` does not exist by design
(`lua/runtime.go:1967`).
- `principal.From(ctx).User` may be a substituted entity ID
(`router.go:465`) or `"unknown"` when unstamped. Store the resolved value;
refuse rather than persist `"unknown"` — note this also makes `-own` checks
meaningless for unstamped callers, so they must be refused, not defaulted.
- Served-vs-inert follows `appbuild/transitions.go:21-39`: inert with no policy,
**fail closed** when a policy exists but the `acl.Request` is absent.
- `validTopLevelKeys` (`metamodel/loader.go:16`) must gain `comments`
(BUG-5XIN07).
