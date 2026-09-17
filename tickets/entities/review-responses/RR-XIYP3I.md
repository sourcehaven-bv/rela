---
id: RR-XIYP3I
type: review-response
title: 'Anchored document handler leaks entity existence: raw GetEntity 404s before the ACL gate with a distinguishable body'
finding: 'api_v1.go:2591 reads the store raw and 404s at :2593 with code entity_not_found and title ''entity "X" not found'' (echoing the id) BEFORE gateReadOrNotFound runs at :2602. A denied id instead gets code not_found and title entityNotFoundTitle ("Entity not found"). The two 404s differ in error code, title, and whether the id is echoed, so a principal who may not read tickets can enumerate which ticket ids exist. This directly violates entityNotFoundTitle''s godoc (api_v1.go:856-859): ''Any handler that 404s on the read path MUST use this const, not a fresh literal, or the bodies drift and existence leaks'' (RR-NGMI). There is also a timing channel: the raw GetEntity is a full store roundtrip for an existing-but-denied id vs a miss for a nonexistent one, the side channel handleV1GetEntity deliberately avoids by gating before the read. TestAnchoredDocument_GateOrderingNoTypeOracle does not catch it because both its probe ids exist, so both take the gate branch; it pins the type oracle, not the existence oracle. Pre-existing, but TKT-K7J6FL extracts exactly this chain into a shared helper, which would carry the oracle onto a second route.'
severity: significant
resolution: 'Scope decision: fix within TKT-K7J6FL rather than splitting into a separate bug. Extracting a known existence oracle into a shared helper and shipping it on a second route is worse than the one-line fix, and any split would land the fix here anyway. resolveAnchoredDocument gates before the raw GetEntity and uses entityNotFoundTitle with no id echo, so both the render and export routes are closed at once. Implementation pending.'
status: addressed
---

## Verification

Confirmed independently against the code, not taken on the reviewer's word.

`internal/dataentry/api_v1.go:2591-2596`:

```go
ent, entErr := a.store.GetEntity(r.Context(), entityID)   // raw, ungated
if entErr != nil {
    writeV1Error(w, r, http.StatusNotFound, "entity_not_found",
        fmt.Sprintf("entity %q not found", entityID), "")
    return
}
if !a.gateReadOrNotFound(w, r, docCfg.EntityType, entityID) { // :2602, runs AFTER
```

A repo-wide grep for `entity_not_found` returns three sites. The two in
`views_handler.go` use the plain title `"Entity not found"`. `api_v1.go:2593` is
the **only** read-path 404 in the package that interpolates the entity id into
the body.

| Probe | Code | Title |
|---|---|---|
| id does not exist | `entity_not_found` | `entity "TKT-999" not found` |
| id exists, read denied | `not_found` | `Entity not found` |

Distinguishable on all three of: error code, title text, id echo.

## Resolution

Fix during the extraction rather than duplicate it. In
`resolveAnchoredDocument`:

1. Move `gateReadOrNotFound` **above** the `GetEntity` call, matching
`handleV1GetEntity`, which gates first precisely to spend the same roundtrip on
a hidden and a nonexistent id (RR-NGMI).
2. Replace the fresh literal with
`writeV1Error(w, r, 404, "not_found", entityNotFoundTitle, "")` — no id echo.
3. Keep the type-mismatch check below the gate so
`TestAnchoredDocument_GateOrderingNoTypeOracle` continues to pass.

New test: assert hidden-vs-absent bodies are byte-identical (minus `instance`)
on **both** the render and the export route.
