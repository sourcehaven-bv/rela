# dataentry — rules for new code

The data-entry web app (Go API + Vue 3 SPA in `frontend/`). Two rule sets
apply here: the write-validation policy, and the `_actions` affordance
contract.

## Validation policy for write APIs

rela's storage is permissive: markdown + YAML frontmatter, edited freely by
external tools alongside the API. Philosophy: **tolerate temporarily invalid
data**; the `analyze_*` tools surface inconsistencies the storage layer
doesn't reject.

Write-time checks split into three classes (DEC-HWZHA):

| Class | When | HTTP |
|---|---|---|
| **Hard 400 — malformed wire format** | Request structure broken, detectable without the metamodel | 400 |
| **Hard 422 — structural impossibility** | Storage layer literally cannot persist this | 422 |
| **Write-with-warnings** | Soft conditions: target type mismatch, missing target, unknown/required-unset/mistyped meta keys | 200 |

The 200 path performs the write and returns warnings `{code, path, detail}`
in the body — `code` matches the corresponding `analyze_*` finding code so
UIs de-duplicate against analyze runs.

**Resist drift toward hard rejection on soft conditions.** Before adding a
422 on a write path, ask: *could a hand-editor produce this state in a
markdown file?* If yes, it's soft — warn, don't reject. JSON:API-style
"validate-then-422" assumes wire and storage share a closed schema; rela's
storage is intentionally more permissive.

## Action affordances (`_actions`)

Every entity and list response carries `_actions: map[string]bool`. The SPA
reads it to decide which write controls to render. The map is a **UI hint** —
the server re-authorizes every write.

Rules for new write code here:

- **Route every `acl.WriteRequest{Op:...}` through `translateVerb`** in
  `affordances.go`. A grep test (`lint_test.go`) enforces it: no other file
  in this package may construct the literal. The shared constructor is the
  structural guarantee that the affordance map and the actual write resolve
  to the same ACL request.
- **Don't trust `_actions` for authorization.** The write endpoint must
  re-authorize. `affordances_contract_test.go` pins the invariant: every
  `_actions[v] == false` ⇒ 403 on the write, every `true` ⇒ 2xx.
- **New verbs require coordinated changes:** add an `acl.Op` constant, a
  `translateVerb` case, and update `docs/data-entry/api-reference.md`. Old
  SPAs ignore unknown keys; removing/renaming a verb is a major API bump.
- **Phase 1 verbs:** `create` (per-collection), `update`/`delete`/`rename`
  (per-item). `transition:*` and `relation:*` are deferred until ACL gains
  Op variants or extension fields.

Rules for new write affordances in the Vue SPA (`frontend/`):

- **Gate every entity-CRUD button** on `entity._actions?.[verb] !== false`
  (or `listResponse._actions?.create !== false` for collection verbs). False
  → hide; anything else (true/undefined/absent) → render. Absent is the
  defensive-render fallback for non-data-entry callers; the server still 403s.
- **No `useACL()` composable or client-side ACL evaluator.** TKT-AWM6L's
  wont-fix rejected this. The SPA reads booleans the server computed — no
  computation, merging, or prediction.
- **Adding a write affordance** requires (a) a backend `translateVerb` entry
  plus a `perItemVerbs`/`perCollectionVerbs` update, and (b) the inline
  `v-if` on the component. No ESLint enforcement; code review catches drift.
