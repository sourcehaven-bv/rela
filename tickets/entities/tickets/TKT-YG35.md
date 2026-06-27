---
id: TKT-YG35
type: ticket
title: 'acl v1 wiring: appbuild + dataentry middleware + SSE audit + docs (PR 4)'
kind: enhancement
priority: high
effort: l
status: done
---

## Summary

Land the end-to-end ACL v1 wiring on top of PRs 1–3: appbuild builds the
Declarative with a real store-backed Graph (replacing PR-2/PR-3 NullGraph
stubs), dataentry attaches a per-request `acl.Request` to ctx so the affordance
resolver actually reuses it, malformed acl.yaml fails boot loudly (RR-72OJ),
`docs/security.md` gets the member-of hardening note (RR-7O6Q), and the SSE
event bridge is audited + pinned so audit-attribution data can never leak via
the event stream. Plus end-user docs (concept + guide entities in rela-docs)
with mermaid conceptual + sequence diagrams of the ACL system.

**PR 4 of 4** in the ACL v1 split. Stacked on PR 3
([#910](https://github.com/sourcehaven-bv/rela/pull/910) →
`feat/affordances-acl-declarative` → TKT-2F2B), which itself stacks on PR 2
([#905](https://github.com/sourcehaven-bv/rela/pull/905)) → PR 1
([#903](https://github.com/sourcehaven-bv/rela/pull/903)). Reference branch:
`feat/acl-v1-tkt-svxl` (TKT-SVXL).

## Scope (in)

**Wiring:**

- `appbuild.loadACLPolicy` + `buildACL(policy, store)` split — the
Declarative is constructed *after* the store is open with a real
`acl.NewStoreGraph(st)`.
- `appbuild.Collaborators.Declarative` field; `WithACL` auto-detects
Declarative when passed one and exposes it via the new
`Services.ACLDeclarative()` accessor.
- `dataentry.attachACLRequest` middleware — builds one `acl.Request`
per HTTP request, threads it through `acl.WithRequest(ctx, r)` so PR 3's
affordance resolver finds it in `acl.FromContext`. Respects upstream-attached
Request (RR-8ZGO).
- `dataentry.ResolverServices` interface: swap `ACLPolicy()` for
`ACLDeclarative()`; `ResolverFromProfile` drops the local NullGraph fallback PR
3 left in place.
- `appbuildtest.WithDeclarative` for test wiring (RR-FGJR).

**Safety:**

- **RR-72OJ:** malformed acl.yaml fails boot loudly. The PR-2
tolerate-warn-on-parse-failure path is replaced; the operator wrote a policy and
the resolver couldn't accept it.
- **SSE audit:** regression test pinning that audit rows never flow
through the SSE broker (so `denied-write` Subject.ID/FromID can't leak to event
subscribers); godoc on `startStoreEventBridge` documents the audit-isolation
invariant; same-origin + loopback middleware gates verified on `/api/events` and
`/api/v1/_events`.

**Docs (in rela-docs):**

- New `concept` entity `CON-authorization` — what the ACL system is,
vocabulary (Subject, Source, Request, Decision, attribution).
- New `guide` entity `GUIDE-acl-overview` — operator's overview with
a **mermaid conceptual diagram** (policy + graph → resolver → decision) and a
**mermaid sequence diagram** (HTTP → middleware → entitymanager → AuthorizeWrite
→ resolver → decision → audit). Worked acl.yaml example. How to read a deny.
- New `guide` entity `GUIDE-acl-security` — member-of hardening
(RR-7O6Q), why operators must validate `member-of` relations come from
authoritative source, why nil Subject panics, why malformed acl.yaml fails boot.
- Relations: `GUIDE-acl-overview --explains--> CON-authorization`;
`GUIDE-acl-security --explains--> CON-authorization`; `GUIDE-acl-overview
--covers--> FEAT-AESD4` (in rela-docs); `GUIDE-acl-security --prerequisite-->
GUIDE-acl-overview`.

## Scope (out)

- Read-side gating on list queries (deferred per source-branch plan).
- Per-link verdict customisation (deferred — see RelationSubject
godoc).
- Mermaid renderer changes — **already supported** via
`internal/htmlutil/mermaid.go` + frontend; just author ```mermaid blocks in the
docs entities.

## Acceptance criteria

1. Malformed `acl.yaml` fails `appbuild.Discover` / `New` with a
wrapped error. *Test:* `TestDiscover_MalformedACL_FailsBoot`.
2. `appbuild.Services.ACLDeclarative()` returns the same
`*acl.Declarative` the Manager uses. *Test:*
`TestNew_WithDeclarative_WiresBothACLAndDeclarative`.
3. `dataentry.attachACLRequest` middleware attaches a Request; the
downstream affordance resolver reuses it (zero re-walk). *Test:* middleware test
asserts `acl.FromContext(r.Context())` is non-nil for protected routes.
4. SSE bridge never emits audit records. *Test:*
`TestSSE_DoesNotFlowAuditEvents` — pump a denied-write through entitymanager;
subscribe to SSE; assert no event for it.
5. Same-origin + loopback middleware still wrap `/api/events` and
`/api/v1/_events`. *Test:* `TestSSE_Endpoints_RequireSameOrigin`.
6. Docs entities created and pass `analyze_cardinality` +
`analyze_validations`. Rendered output includes mermaid SVGs (or at minimum the
```mermaid block is preserved through goldmark → ConvertMermaidBlocks for the
frontend mermaid.js to render).
7. Full tree green; race-clean; lint clean; arch-lint clean.

## RRs incorporated

| RR | Severity | What lands |
|----|----------|------------|
| RR-72OJ | critical | fail boot on malformed acl.yaml |
| RR-FGJR | significant | `Collaborators.Declarative` + `appbuildtest.WithDeclarative` |
| RR-36UL | significant | `WithACL` auto-detect Declarative |
| RR-JJYW (middleware part) | significant | `attachACLRequest` |
| RR-8ZGO | minor | middleware respects upstream Request |
| RR-7O6Q | nit | member-of hardening doc (rela-docs guide) |

## Notes

- PR-2 stub `appbuild.loadACL` (NullGraph fallback +
tolerate-warn-on-parse) is **replaced**, not appended to. Same for the PR-3
`ResolverFromProfile` local-Declarative-with-NullGraph build. Both were
placeholders for this PR.
- SSE audit task surfaced no existing leaks (the bridge emits only
`{type, id}` markers, no payloads or audit data). The plug is defensive: a
regression test and a godoc invariant so a future change can't unintentionally
leak.
- Mermaid is already rendered end-to-end (`internal/htmlutil/mermaid.go`
  + `frontend/src/utils/markdown.ts`). No generator changes needed.
```
