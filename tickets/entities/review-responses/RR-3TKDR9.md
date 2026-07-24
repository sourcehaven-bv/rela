---
id: RR-3TKDR9
type: review-response
title: Adding a slice field to Principal breaks == comparison in mcp.NewServer guard
finding: internal/mcp/server.go:166 does `s.principal == (principal.Principal{})`. Principal is currently comparable (three string fields); the plan adds AssertedRoles []string, and slices are not comparable, so this stops compiling. The file is absent from the plan's Files to modify list. The build break is benign; the hazard is the reflexive fix — reflect.DeepEqual would silently change the guard's meaning so a Principal carrying only asserted roles would no longer count as zero.
severity: critical
resolution: 'Confirmed: mcp/server.go:166 does `s.principal == (principal.Principal{})` and Principal becomes non-comparable once it carries a slice field (unexported per RR-TQBZ2U, but unexported slices still break ==). Resolution: add an explicit IsZero() method on Principal and replace the comparison with `s.principal.IsZero()`. This is now unambiguous rather than a judgement call under compile pressure — the RR-TQBZ2U constructor decision means the semantics are defined in one place. Explicitly NOT reflect.DeepEqual, which would silently change the guard so a Principal carrying only roles no longer counts as zero. internal/mcp/server.go added to the plan''s Files to modify. Test: a Principal carrying only asserted roles still fails the NewServer guard.'
status: addressed
---

## Finding

`internal/mcp/server.go:166`:

```go
if s.principal == (principal.Principal{}) {
    return nil, errors.New("mcp.NewServer: Principal is required (use WithPrincipal)")
}
```

`principal.Principal` is currently comparable (three string fields). The plan
adds `AssertedRoles []string` — **slices are not comparable, so this stops
compiling.** `internal/mcp/server.go` is absent from the plan's "Files to
modify" list.

## Impact

The build break is the benign half. The hazard is the reflexive fix under time
pressure:

- `reflect.DeepEqual` — a silent behaviour change; a Principal carrying only
`AssertedRoles` would no longer be "zero", so a caller that set roles but no
User/Tool would pass a guard designed to reject it.
- `s.principal.User == "" && s.principal.Tool == ""` — correct, but must be a
deliberate choice with a test, not a compile-error patch.

## Resolution

Make the replacement an explicit plan step with the chosen predicate stated, and
add a test that a Principal carrying **only** asserted roles still fails the
guard. 87 `principal.Principal{...}` composite literals exist repo-wide (21 in
non-test code); verify none other relies on comparability or uses Principal as a
map key before implementing.
