---
id: RR-R1I6AD
type: review-response
title: _schema is the wrong endpoint; _config already carries the form data and is pinned principal-independent
finding: 'Three problems with putting create_form + _actions on _schema. (1) handleV1Schema (api_v1.go:1148) builds from a.State().Meta only and has no access to createFormForType, a *viewsHandler method — making it reachable is a config-into-metamodel layering leak, not a one-liner. (2) The plan mitigates "HTTP caching" but the real cache is client-side: schema.ts:216 `if (loaded.value) return` loads once per session and never revalidates. (3) _config already ships every Form with entity_type and mode, so condition B is derivable client-side with zero server change. Note _config is pinned principal-independent by nav_permission_test.go:346, so it cannot carry the ACL boolean.'
severity: critical
resolution: Signals split by layer. Condition B derived client-side from the _config forms map (entity_type + mode) already on the wire — zero server change, no metamodel/config leak. Condition A ships as an explicitly-named per-type create map on a principal-scoped payload, not on _schema (pure metamodel projection, no resolver access) and not on _config (pinned principal-independent by nav_permission_test.go:346). Client session cache documented as fail-safe.
status: addressed
---

## Resolution

Split the two signals by which layer owns them:

- **Condition B (a form exists)** — derived **client-side** from the `_config`
forms map already on the wire (`entity_type` + `mode`). Zero server change, no
layering violation. The client mirrors `createFormForType`'s ordering (natsort,
prefer non-edit, fall back to edit) so the SPA and the side-panel resolver
agree; a shared Go test pins the ordering the client replicates.
- **Condition A (may create type X)** — the only genuinely new server value.
It cannot go on `_config` (pinned principal-independent) and does not belong on
`_schema` (metamodel projection). It goes on its own principal-scoped read: a
`create` map on the existing sidebar/affordance payload, which is already
per-principal, computed with `computeCollectionActions` verbatim.

This halves the server change and removes the metamodel/config leak.

The client-side session cache is documented as acceptable: `_actions` is a UI
hint that the POST re-authorizes, so a stale map can only show a button that
then 403s — never grant anything. Noted in docs rather than mitigated.
