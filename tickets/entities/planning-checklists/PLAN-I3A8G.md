---
id: PLAN-I3A8G
type: planning-checklist
title: 'Planning: Soften workspace write validation per DEC-HWZHA'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**
- Reclassify `metamodel.ValidateProperties` errors at the workspace boundary: `ValidationErrorRequired`, `ValidationErrorInvalidType`, AND `ValidationErrorInvalidValue` (per RR-3C82L) stop being hard 422s and become warnings instead. `ValidateEntity`'s `ValidationErrorIDPrefix` and `ValidationErrorUnknownType` errors stay hard 422 (structural impossibilities — the file path can't be constructed).
- `entitymanager.CreateResult` and `entitymanager.UpdateResult` gain `Warnings []Warning` (matching the type TKT-6WLSW added).
- `workspace.createEntity` and `workspace.updateEntity` return `(*Result{Warnings: [...]}, nil)` for soft conditions instead of `(nil, *ValidationError)`. The hard structural errors still return `(nil, *ValidationError)` and propagate as 422.
- `entitymanager.CreateRelation` and `UpdateRelation` results gain `Warnings []Warning` too (per RR-FX5GZ — keep Lua surface consistent across entity and relation writes).
- All callers migrate in the same PR: dataentry HTTP handlers, MCP entity tool, CLI commands, Lua bindings.
- `PATCH /api/v1/{plural}/{id}` and `POST /api/v1/{plural}` surface the warnings array in the response body (the wire shape already exists from TKT-6WLSW; this ticket actually populates it for entity-level findings).
- MCP create_entity / update_entity tool result text gains a "**WARNINGS** (n)" first-line section per warning, before any other content (per RR-R53U9 — make warnings programmatically discoverable). `isError` stays false (the write succeeded — that's the truth).
- CLI `create` / `update` / `set` commands print warnings to stderr with `WARNING: <code> at <path>: <detail>` lines; exit code 0 unless `--strict` is set (per RR-3SE7A).
- `--strict` flag added to `rela create` / `update` / `set`: when set, ANY warnings cause exit code 1 with the same stderr output. Default exit code 0.
- Lua `rela.create_entity` / `rela.update_entity` / `rela.create_relation` / `rela.update_relation` add a **second return value** (per RR-FX5GZ — entity AND relation, for surface consistency) carrying the warnings table. The first return value is unchanged so existing scripts that ignore warnings continue to work without code changes. **Contract: returns `(value, warnings)` like `string.gsub`, NOT `(value, error)` like `io.open`** (per RR-7L18Y — both can be non-nil simultaneously; warnings is `nil` not empty when there are none; hard failures still raise via `RaiseError`).
- `IsSoft()` lives on `*metamodel.ValidationError` in `internal/metamodel/validation.go` (per RR-30U7K — categorization is property of error category, not workspace policy).
- All existing tests that assert 422 on validation soft conditions flip to assert 200 + warning. New tests cover the type-mismatch, invalid-value, and (unchanged) hard-422 cases.
- `docs/data-entry/api-reference.md` documents the new entity-level warning codes.
- `docs/lua-scripting.md` (or wherever Lua docs live) documents the multi-return contract with explicit "string.gsub-style, not io.open-style" framing.
- `CLAUDE.md` notes are unchanged in spirit — Validation policy section already lays out the three classes; this ticket just makes the workspace conform.

**Out of scope:**
- **Closed-schema check on input property keys** (per RR-R03O6): `ValidationErrorUnknownProperty` does NOT exist today. `ValidateProperties` only walks declared keys; unknown input keys are silently accepted. Adding a closed-schema check is its own design (do unknown keys warn? are they tolerated for forward-compat / migration?). Defer to separate ticket. Plan no longer references `unknown_property_key` warning code — that doesn't have a behavior to soften.
- Softening relation-write validation beyond what the modern PATCH already does (TKT-6WLSW). Per-edge endpoints stay as-is until TKT-ZEKO4 retires them.
- A `rela.validate(...)` Lua API. **Note**: `rela validate` CLI command already exists (`internal/cli/validate.go`) — that's the strict-validation entry point for CI scripts (per RR-3TIFL). Lua-side strict validation is a follow-up ticket.
- Frontend UI changes to display warnings inline. TKT-E6094 (autosave) consumes warnings naturally; other UI surfaces gain warnings ad-hoc.
- Migration of analyze tools — they're appropriate places for hard checks.
- Backwards-compat shim for unmigrated callers. Every caller migrates in this PR; the compiler catches anyone we missed.
- Auto-save UX warning fatigue mitigation (per RR-5V6CN). Suppressing warnings to soften UI noise is data-hiding at the API boundary. Solve in TKT-E6094 (debounce, dismiss, show-only-new) — not here.

**Acceptance Criteria:**

### Backend (workspace + entitymanager) — softened categories

1. **AC1 — Soften required-field-missing**: `workspace.updateEntity` for an entity with a required property cleared returns `(*UpdateResult{Entity: e, Warnings: [{code: "required_property_unset", path: "/properties/<name>", detail: "This field is required"}]}, nil)`. Entity is persisted with the missing field. **Test**: workspace test.
2. **AC2 — Soften type mismatch**: PATCH with `weight: "not-a-number"` for an integer property returns `Warnings: [{code: "property_type_mismatch", path: "/properties/weight", detail: "Must be an integer"}]`. Entity is persisted with the wrong-typed value. **Test**: workspace test.
3. **AC3 — Soften invalid value (enum)**: PATCH with an enum property set to a value not in the enum's allowlist returns `Warnings: [{code: "property_value_invalid", path: "/properties/<name>", detail: "..."}]`. Entity is persisted. **Test**: workspace test.
4. **AC4 — Soften invalid value (date)**: PATCH with a date property set to `"2026-13-99"` returns `Warnings: [{code: "property_value_invalid", path: "/properties/<name>", ...}]`. Entity is persisted. **Test**: workspace test.
5. **AC5 — Soften invalid value (RRULE)**: PATCH with an RRULE property set to malformed RRULE returns `Warnings: [{code: "property_value_invalid", ...}]`. Entity is persisted. **Test**: workspace test.

### Backend — hard structural errors stay 422

6. **AC6 — Hard 422 on unknown entity type**: `workspace.updateEntity` for an entity whose type isn't in the metamodel returns `(nil, *ValidationError)`. **Test**: workspace test.
7. **AC7 — Hard 422 on bad ID prefix**: An entity ID that doesn't match any of the type's declared prefixes returns `(nil, *ValidationError)`. **Test**: workspace test.

### Backend — Create + result types

8. **AC8 — Same softening for `workspace.createEntity`**: AC1, AC2, AC3 mirror at create time. **Test**: workspace test.
9. **AC9 — `entitymanager.CreateResult` and `UpdateResult` carry warnings**: field is `Warnings []Warning`, omitempty in JSON, with the same shape as TKT-6WLSW's `Warning` (`{code, path, detail}`). **Test**: Go test on the JSON shape.
10. **AC10 — Multiple soft conditions on one entity**: clear a required field AND set an invalid enum value in one PATCH. Response includes BOTH warnings. Both persisted. **Test**: workspace test.
11. **AC11 — Mixed hard + soft**: clear a required field on an entity with a bad ID prefix. Returns 422 (the hard error wins; warnings are NOT surfaced because the write didn't happen). **Test**: workspace test.

### Backend (dataentry HTTP)

12. **AC12 — `PATCH /api/v1/{plural}/{id}` returns 200 + warnings on required-field-missing**: regression on what's currently 422. Response body's `warnings` carries entity warnings AND any relation warnings (TKT-6WLSW) merged. **Test**: dataentry test.
13. **AC13 — `POST /api/v1/{plural}` returns 201 + warnings on required-field-missing on create**: status stays 201 (created), warnings included. **Test**: dataentry test.

### CLI

14. **AC14 — `rela update` prints warnings to stderr**: `rela update TKT-001 --status=` (clears a required field) prints `WARNING: required_property_unset at /properties/status: ...` to stderr. Exit code 0. **Test**: CLI integration test.
15. **AC15 — `rela create` prints warnings**: same UX. Exit code 0. **Test**: CLI integration test.
16. **AC16 — Hard validation errors still exit 1**: `rela update TKT-001` on an entity with a non-matching prefix exits 1 with the error on stderr. **Test**: CLI integration test.
17. **AC17 — `--strict` elevates warnings to exit-1**: `rela update --strict TKT-001 --status=` prints WARNING to stderr AND exits 1. **Test**: CLI integration test for create, update, set.

### MCP

18. **AC18 — MCP `update_entity` tool result includes warnings as first line**: result starts with `WARNINGS (n):` then per-warning lines, then a separator, then the success message. `isError` is false. **Test**: MCP integration test asserts the warnings prefix is programmatically findable, not just substring-present somewhere in the body.
19. **AC19 — MCP `create_entity` tool likewise**: same prefix format. **Test**: MCP integration test.
20. **AC20 — MCP tool description mentions warnings convention**: the tool's registered description (visible to AI agents) explains "Result text begins with `WARNINGS (n):` when soft validation issues occurred. Check for this prefix to detect them programmatically." **Test**: MCP test asserts description contains the convention text.

### Lua

21. **AC21 — `rela.update_entity` returns `entity, warnings`**: `local e = rela.update_entity(id, {title = "x"})` continues to work — entity is the first return. `local e, warnings = rela.update_entity(id, {title = ""})` (clearing required) gets `e` populated AND `warnings` as a non-nil array. **Test**: Lua integration test.
22. **AC22 — `rela.create_entity` likewise**: same multi-return on create. **Test**: Lua integration test.
23. **AC23 — `rela.create_relation` returns `(relation, warnings)`** (per RR-FX5GZ): same shape. `nil` second-return when no warnings. **Test**: Lua integration test.
24. **AC24 — `rela.update_relation` returns `(relation, warnings)`** if it exists, or this AC is N/A (verify the function exists during implementation; current code has only `create_relation`/`delete_relation` AFAICT). **Test**: Lua integration test (or removed AC).
25. **AC25 — `warnings` is `nil` (not empty table) when there are none**: scripts can use `for _, w in ipairs(warnings or {})` idiomatically. The second return is `nil`, NOT `""`, NOT `{}`. **Test**: Lua integration test (per RR-7L18Y — defends against io.open-style mistake).
26. **AC26 — Hard validation errors still raise**: `pcall(rela.update_entity, "BAD-PREFIX", {})` returns `(false, error_message)`. Hard errors haven't changed channel. **Test**: Lua integration test.
27. **AC27 — Soft validation no longer raises**: `pcall(rela.update_entity, id, {title = ""})` for required-title entity returns `(true, entity, warnings)`. Behavior shift documented. **Test**: Lua integration test.

### Concurrency, JSON pointers, edge cases

28. **AC28 — Concurrent PATCHes on same entity**: two interleaved PATCHes (writeMu serializes). Both responses include warnings reflecting THEIR respective merged in-memory entity state at validation time, not on-disk state at response-construction time. **Test**: dataentry test with two goroutines (per RR-61JFH).
29. **AC29 — Property name with `/` in path is RFC 6901-escaped**: synthetic test with a property named `foo/bar` (or skip if metamodel loader rejects such names; document either way). **Test**: workspace test if accepted, otherwise plan note (per RR-Z19ME).
30. **AC30 — Required boolean=false regression**: entity with required boolean property=false; save, reload, PATCH unrelated property; assert NO `required_property_unset` warning for the boolean. If this fails, file the storage-omitempty bug separately as out-of-scope. **Test**: workspace test (per RR-C7TE6).

### Documentation

31. **AC31 — `docs/data-entry/api-reference.md` documents the three new warning codes**: `required_property_unset`, `property_type_mismatch`, `property_value_invalid`. **Test**: doc grep in CI (or manual).
32. **AC32 — Lua scripting docs document multi-return contract**: explicit "string.gsub-style, not io.open-style" framing, code example for ignoring vs reading warnings (per RR-7L18Y). **Test**: doc grep.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **DEC-HWZHA** is the canonical reference: three classes, what stays 4xx vs warns. This ticket is the validation-layer migration the decision's "Migration" section schedules separately.
- **TKT-6WLSW**: shipped the wire format `warnings: [{code, path, detail}]` with RFC 6901 paths and the exact policy split. The relation reconciler's existing classification is the template; this ticket applies the same classification at the entity validator boundary.
- **In-tree ValidateProperties** (`internal/metamodel/validation.go:36`): five error categories — `ValidationErrorRequired`, `ValidationErrorInvalidValue`, `ValidationErrorInvalidType`, `ValidationErrorUnknownType`, `ValidationErrorIDPrefix`. The first three soften (per RR-3C82L); the last two stay 422. `ValidationErrorUnknownProperty` does NOT exist (per RR-R03O6) — closed-schema check is out of scope.
- **`workspace.updateEntity` / `createEntity`** (`internal/workspace/workspace.go:836, 990`): both call `meta.ValidateEntity` and return `newValidationError(errs)` on any error. This is the single bottleneck where the policy decision lives — change here, ripple through callers.
- **No existing analyze code vocabulary for built-in property validations**: `validator.Violation` has `RuleName` (user-defined rule name); built-in property validation is only enforced at write time today. So the codes this ticket defines ARE the canonical codes — no analyze-side codes to align with (resolves the question raised in RR-BW181).
- **`internal/cli/validate.go`** exists (per RR-3TIFL) — `rela validate` is the strict-validation entry point for CI scripts. CLI `--strict` flag on create/update/set is the per-command escape hatch (per RR-3SE7A).
- **Multi-return convention in Lua**: `string.gsub` returns `(s, count)` where both are always populated; `io.open` returns `(file, err)` where they're mutually exclusive. This ticket follows the **`string.gsub` pattern** — both can be non-nil simultaneously, the second is additional success info, not error info (per RR-7L18Y, must be documented to prevent users from applying the io.open mental model).
- **Existing in-tree script `tickets/scripts/stale-review.lua`** calls `rela.update_entity` 3 times (lines 187, 195, 207), no `pcall` wrapping, return values ignored. **Verified**: post-ticket continues to work identically — first return is the entity table as before, second return is silently dropped (per RR-VMDWD).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**Layer 0 — Classify validation errors** (`internal/metamodel/validation.go`):

Add `IsSoft()` on `*ValidationError` (per RR-30U7K — keep classification close
to the type definition):

```go
// IsSoft reports whether the error is a soft condition per DEC-HWZHA — a
// state a hand-edited markdown file could produce that the API should
// tolerate at write time and surface as a warning rather than a 422.
func (e *ValidationError) IsSoft() bool {
    switch e.Type {
    case ValidationErrorRequired,
         ValidationErrorInvalidType,
         ValidationErrorInvalidValue:
        return true
    }
    return false
}
```

`ValidationErrorUnknownType` and `ValidationErrorIDPrefix` return false
(structural — file path can't be constructed).

Test in `validation_test.go`: table test enumerating every type returns the
expected classification.

**Layer 1 — Workspace boundary** (`internal/workspace/workspace.go`):

In both `createEntity` (line 990) and `updateEntity` (line 836), partition the
validator's errors:

```go
errs := w.meta.ValidateEntity(e.ID, e.Type, e.Properties)
var hardErrs []*metamodel.ValidationError
var warnings []entitymanager.Warning
for _, err := range errs {
    if err.IsSoft() {
        warnings = append(warnings, entitymanager.Warning{
            Code:   warningCodeFor(err.Type),
            Path:   "/properties/" + jsonPointerEscape(err.Property),
            Detail: err.Message,
        })
    } else {
        hardErrs = append(hardErrs, err)
    }
}
if len(hardErrs) > 0 {
    return nil, newValidationError(hardErrs)
}
// proceed with write — warnings ride on the result
```

`warningCodeFor` is a small mapping (table inline in the workspace package,
since the codes are policy-layer concerns):

| ValidationErrorType | Warning code |
|---|---|
| `ValidationErrorRequired` | `required_property_unset` |
| `ValidationErrorInvalidType` | `property_type_mismatch` |
| `ValidationErrorInvalidValue` | `property_value_invalid` |

Note: this is full-state validation (per RR-5V6CN). The validator sees the
merged entity; warnings reflect what's wrong with the entity as it will be
persisted. No per-PATCH scoping of warnings.

**Layer 2 — `entitymanager` result types**
(`internal/entitymanager/entitymanager.go`):

```go
type CreateResult struct {
    Entity   *entity.Entity
    Warnings []Warning      // NEW
}

type UpdateResult struct {
    Entity   *entity.Entity
    Warnings []Warning      // NEW
}
```

Per RR-FX5GZ, also update relation results:

```go
type CreateRelationResult struct {
    Relation *entity.Relation
    Warnings []Warning      // NEW
}
```

(Verify `CreateRelation`/`UpdateRelation` currently return `(*entity.Relation,
error)` and we need to introduce a result type. If the existing surface is too
entrenched to change easily, keep the relation interface as-is and only thread
warnings into the Lua binding via the workspace's internal struct.)

The `wsEntityManager` adapter (`internal/workspace/manager.go`) propagates
warnings from the workspace's internal results to the manager interface's
same-named types.

**Layer 3 — Caller migrations** (one PR):

| File | Today | After |
|---|---|---|
| `internal/dataentry/api_v1.go` `handleV1UpdateEntity` | calls `a.em.UpdateEntity(...)`, expects 422 | merges `result.Warnings` into the response's `Warnings` array (existing field from TKT-6WLSW) |
| `internal/dataentry/api_v1.go` create paths | calls `a.em.CreateEntity(...)`, expects 422 | same merge |
| `internal/dataentry/handlers_api.go` legacy POST/PATCH | similar | similar |
| `internal/mcp/tools_entity.go` `handleCreateEntity` / `handleUpdateEntity` | tool result is just success / error | result text begins with `WARNINGS (n):\n  <code>: <detail> (<path>)\n  ...\n---\n` then existing success text. `isError` stays false. |
| `internal/cli/create.go`, `update.go`, `set.go` | error → `os.Exit(1)` | print warnings to stderr in `WARNING: <code> at <path>: <detail>` format; exit 0 unless `--strict` flag → exit 1 |
| `internal/lua/runtime.go` `luaCreateEntity` / `luaUpdateEntity` | push `EntityToTable(result.Entity)`, return 1 | also push `WarningsToTable(result.Warnings)` (or `lua.LNil` when `len == 0`), return 2 |
| `internal/lua/runtime.go` `luaCreateRelation` (if exists) | similar | similar |

`WarningsToTable` is a new helper next to `EntityToTable`:

```go
// WarningsToTable converts a slice of Warning to a Lua table. Returns
// LNil (not an empty table) when the slice is empty so scripts can
// use `for _, w in ipairs(warnings or {})` idiomatically.
func WarningsToTable(ls *lua.LState, warnings []entitymanager.Warning) lua.LValue {
    if len(warnings) == 0 {
        return lua.LNil
    }
    tbl := ls.NewTable()
    for _, w := range warnings {
        wt := ls.NewTable()
        ls.SetField(wt, "code", lua.LString(w.Code))
        ls.SetField(wt, "path", lua.LString(w.Path))
        ls.SetField(wt, "detail", lua.LString(w.Detail))
        tbl.Append(wt)
    }
    return tbl
}
```

**Layer 4 — `--strict` CLI flag** (per RR-3SE7A):

In each of `internal/cli/create.go`, `update.go`, `set.go`:

```go
strict := false
cmd.Flags().BoolVar(&strict, "strict", false,
    "exit with status 1 if soft validation warnings are surfaced")

// after the call:
for _, w := range result.Warnings {
    fmt.Fprintf(os.Stderr, "WARNING: %s at %s: %s\n", w.Code, w.Path, w.Detail)
}
if strict && len(result.Warnings) > 0 {
    os.Exit(1)
}
```

Document on each command's `--help`: "Use --strict to fail the command on soft
validation warnings."

**Layer 5 — MCP tool result rendering** (per RR-R53U9):

In `internal/mcp/tools_entity.go`, build the result text with warnings as the
leading section:

```go
var sb strings.Builder
if len(result.Warnings) > 0 {
    fmt.Fprintf(&sb, "WARNINGS (%d):\n", len(result.Warnings))
    for _, w := range result.Warnings {
        fmt.Fprintf(&sb, "  %s: %s (%s)\n", w.Code, w.Detail, w.Path)
    }
    sb.WriteString("---\n")
}
fmt.Fprintf(&sb, "Updated entity %s", result.Entity.ID)
return mcp.NewToolResultText(sb.String()), nil
```

Update the tool registration's description to include:

```
Result text begins with "WARNINGS (n):" when soft validation
issues occurred (e.g., required field missing, type mismatch).
Check for this prefix to detect them programmatically. Hard
errors (unknown entity type, bad ID prefix) still come back via
the standard MCP error channel.
```

**Layer 6 — Tests**:

- `internal/metamodel/validation_test.go` — table test for `IsSoft()` covering every type.
- `internal/workspace/workspace_test.go` — softened cases (AC1–5, AC8, AC10), hard cases (AC6, AC7, AC11), result-shape (AC9), boolean-false regression (AC30).
- `internal/dataentry/api_v1_test.go` — HTTP cases (AC12, AC13), concurrency (AC28), JSON pointer escape if applicable (AC29).
- `internal/cli/*_test.go` — CLI cases (AC14–17).
- `internal/mcp/tools_entity_test.go` — MCP cases (AC18–20).
- `internal/lua/runtime_test.go` — Lua cases (AC21–27).

**Layer 7 — Documentation**:

- `docs/data-entry/api-reference.md` — extend the warning-code table with the three new codes (AC31).
- `docs/lua-scripting.md` (or wherever) — multi-return contract section with explicit "string.gsub-style, not io.open-style" framing and code examples (AC32). Cover entity AND relation Lua APIs.
- `CLAUDE.md` — already covers the policy; verify the example codes match.

**Files to modify:**

- `internal/metamodel/validation.go` — add `IsSoft()`
- `internal/metamodel/validation_test.go` — table test for IsSoft
- `internal/workspace/workspace.go` — partition errors in `createEntity` / `updateEntity`; populate `Warnings`; add `warningCodeFor` mapping
- `internal/workspace/workspace_test.go` — new ACs
- `internal/workspace/manager.go` — adapter passes Warnings through
- `internal/entitymanager/entitymanager.go` — add `Warnings []Warning` to `CreateResult` / `UpdateResult` (and relation result type if reasonable)
- `internal/dataentry/api_v1.go` — merge entity warnings into response
- `internal/dataentry/api_v1_test.go` — AC12, AC13, AC28, AC29
- `internal/dataentry/handlers_api.go` — same for legacy endpoints
- `internal/mcp/tools_entity.go` — render warnings section + tool description update
- `internal/mcp/tools_entity_test.go` — AC18, AC19, AC20
- `internal/cli/create.go`, `update.go`, `set.go` — print warnings + `--strict` flag
- `internal/cli/*_test.go` — AC14–17
- `internal/lua/runtime.go` — multi-return + `WarningsToTable` helper
- `internal/lua/runtime_test.go` — AC21–27
- `docs/data-entry/api-reference.md` — new warning codes
- `docs/lua-scripting.md` — multi-return contract documentation

**Alternatives considered:**

- **Backwards-compat shim**: rejected. Every caller migrates in this PR; the compiler catches anyone we missed.
- **Add the closed-schema (`unknown_property_key`) check in this ticket**: rejected per RR-R03O6. The check doesn't exist today, so adding it isn't softening — it's net-new behavior with its own design questions (forward-compat tolerance, etc.). Defer to separate ticket.
- **Lua: change first return to `{entity, warnings}` table**: rejected. Breaks every existing script's `e.id` access pattern. Multi-return preserves backwards compat in script code.
- **Lua: return empty table instead of nil for warnings**: rejected per RR-7L18Y. Lua convention treats nil-or-empty as equivalent via `for _, w in ipairs(warnings or {})`; nil is more honest for "nothing to say" and avoids the io.open-style "second return non-nil means problem" misinterpretation (empty `{}` is truthy in Lua).
- **Re-elevate warnings to errors via a `strict` option on the manager**: rejected. CLI gets `--strict` flag for the per-command case (per RR-3SE7A). MCP/Lua callers can check `len(warnings) > 0` themselves. `rela validate` exists for CI strict-check.
- **CLI: exit code 2 for "succeeded with warnings"**: rejected. Not standard; confuses scripting consumers. Exit 0 + warnings to stderr matches `make`, `go build`, `npm install`. `--strict` provides the escape hatch.
- **MCP: return warnings via a structured `meta` field on tool result**: explored. The MCP SDK we use doesn't expose `_meta` cleanly via `mcp.NewToolResultText`. Sentinel-prefix-in-text approach (RR-R53U9 option b) achieves programmatic discoverability with minimal SDK surface, plus the tool description primes agents.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Property values from API/CLI/Lua**: already validated by `ValidateProperties` today. This ticket reclassifies what we do with the validation result, not what the validator checks. Allowlist nature unchanged.
- **Property names from API/CLI/Lua**: closed-schema check explicitly out of scope (per RR-R03O6). Existing behavior retained: unknown keys silently accepted on write. This ticket does NOT change that.
- **JSON pointers in warnings**: same RFC 6901 escaping as TKT-6WLSW. `jsonPointerEscape` already exists in `internal/dataentry/`. **AC29** verifies escape for property names containing `/` or `~` (or documents that the metamodel loader rejects such names, in which case the test is skipped).

**Security-Sensitive Operations:**

- **No new file-system surface, no new auth surface, no crypto.**
- **Disclosure**: warning details echo property names and the validator's `Message` strings. This is the same surface the existing 422 error response has today. No new leakage.
- **`--strict` flag on CLI**: no security surface. Just an exit-code modifier.

**Error handling:**

- Hard errors (unknown type, bad ID prefix) keep their current 422 path with the existing detail string format. No change.
- Warnings echo the validator's `Message` field, which is already user-facing-safe (e.g., "This field is required", "Must be a string", "Must be one of: low, medium, high").

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios**: AC1–AC32 each map to a specific test in the file noted in
Layer 6.

**Edge Cases:**

- **Multiple soft conditions on one entity** (AC10): clear required + invalid enum value in one PATCH. Both warnings, both persisted.
- **Mixed hard + soft** (AC11): clear required + bad ID prefix. 422 wins; no warnings (no write).
- **Empty warnings array vs nil**: HTTP response uses `omitempty` (absent in JSON); Lua returns `nil` (AC25); Go `result.Warnings` is `nil` (not `[]Warning{}`). Each tested.
- **Property of type `enum` set to a value not in the enum** (AC3): covered by `ValidationErrorInvalidValue` with `property_value_invalid` warning.
- **Required boolean property set to `false`** (AC30): NOT a required-unset condition. Existing isEmptyList/nil-check logic is correct. Regression test catches the storage-omitempty edge case if it exists (file separately if it does).
- **JSON pointer escaping**: AC29.
- **Concurrent PATCHes**: AC28 (two goroutines, writeMu serializes, both responses include warnings reflecting their respective merged state).
- **Order of warnings in a multi-warning response**: stable for clients. The validator returns errors in iteration order over `schema.PropertyDefs()` (Go map iteration is randomized but iteration is per-call deterministic). Sort warnings by `path` in the workspace boundary before emitting, so clients get consistent ordering. **Add to plan: sort warnings by path.**

**Negative Tests:**

- AC6 — unknown entity type → 422.
- AC7 — bad ID prefix → 422.
- AC11 — hard + soft → 422.
- AC16 — CLI hard validation error → exit 1.
- AC26 — Lua hard validation error → raises (pcall returns false).

**Integration test approach:**

- Backend: existing test harnesses in workspace, dataentry, mcp, cli, lua. Each gets the AC-targeted updates above.
- No new e2e tests required — the existing data-entry e2e suite already exercises forms; the contract change at the API level is invisible to those tests as long as they don't depend on 422 specifically.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Risk: Some callers depend on 422 to surface validation issues to users; softening loses that signal.**
   - **Mitigation**: every migrated caller surfaces warnings: HTTP body, MCP tool result with leading WARNINGS prefix (programmatically discoverable per RR-R53U9), CLI stderr (with `--strict` for CI gate per RR-3SE7A), Lua return value. Information IS visible — just doesn't block writes. AC tests assert each surface.
2. **Risk: Lua scripts using `pcall(rela.update_entity, ...)` to detect validation failures silently change behavior.**
   - **Mitigation**: documented migration. **Verified in-repo**: `tickets/scripts/stale-review.lua` doesn't pcall (per RR-VMDWD). User scripts outside this repo are out of scope for audit; the warning surface in the second return is well-documented (AC32 + RR-7L18Y).
3. **Risk: MCP AI clients that previously detected validation failures via `isError` silently get success.**
   - **Mitigation**: tool description (AC20) and result-text leading prefix (AC18) make warnings programmatically discoverable. Agents reading the description are primed to look for `WARNINGS (n):`.
4. **Risk: A hidden caller breaks because the result struct grew.**
   - **Mitigation**: Go's struct extension is additive — adding a field doesn't break code that doesn't reference it.
5. **Risk: Lua user adopts `(entity, warnings)` with io.open mental model and writes `if w then error(w) end`.**
   - **Mitigation**: documented contract (AC32, RR-7L18Y) with explicit "string.gsub-style, NOT io.open-style" framing and a code example. AC25 asserts `nil` (not `""`) so users can't mistake warnings-absent for an empty error message.
6. **Risk: Warning ordering non-deterministic across requests; tests flake.**
   - **Mitigation**: sort warnings by `path` in the workspace boundary before emitting. Tests assert on sorted order.
7. **Risk: Closed-schema check is a behavior gap (unknown keys silently accepted on write).**
   - **Mitigation**: explicitly out-of-scope per RR-R03O6. Filed as a follow-up consideration; this ticket does not regress and does not improve. Document in plan.

**Effort: m** — backend validator and workspace boundary changes are small and
isolated. Caller updates spread across 5 files but mechanical. `--strict` flag
is ~3 lines per CLI command. MCP rendering ~10 lines. Lua bindings ~5 lines per
function plus `WarningsToTable` helper. Tests are the bulk of the work. ~2 days
of focused work.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `docs/data-entry/api-reference.md` adds three new entity-level warning codes (`required_property_unset`, `property_type_mismatch`, `property_value_invalid`)
- [x] CLI help text — `rela create` / `update` / `set` help mentions `--strict` flag
- [x] CLAUDE.md — already covers the policy; verify the example codes match
- [ ] frontend/CLAUDE.md — N/A (no frontend changes here)
- [ ] README.md — N/A
- [x] API docs — `internal/openapi/openapi.yaml` regenerated to reflect the new warning codes
- [x] Lua API docs — multi-return contract documented with explicit "string.gsub-style, not io.open-style" framing. Covers entity AND relation Lua APIs (per RR-FX5GZ).
- [ ] N/A

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **RR-R03O6** (critical, addressed): `ValidationErrorUnknownProperty` doesn't exist; closed-schema check dropped from scope. Out-of-scope section explicitly documents the gap.
- **RR-3C82L** (critical, addressed): `ValidationErrorInvalidValue` added to `IsSoft()` switch. New ACs (AC3, AC4, AC5) cover enum, date, RRULE invalid-value warnings. New warning code `property_value_invalid`.
- **RR-BW181** (significant, addressed): Verified — no existing analyze code vocabulary for built-in property validations. Codes defined in this ticket ARE the canonical codes. Mapping table is the three-class minimum: `property_type_mismatch` (wrong primitive type), `property_value_invalid` (right type, value rejected), `required_property_unset`.
- **RR-3TIFL** (significant, addressed): `rela validate` CLI exists; documented in Out-of-scope section as the strict-validation entry point. Lua-side `rela.validate` follow-up noted but not in scope.
- **RR-5V6CN** (significant, addressed): warning scope LOCKED to "post-write entity full state" (no per-PATCH scoping). Risk #5 removed; Out-of-scope notes that auto-save warning fatigue is TKT-E6094's problem to solve in UI.
- **RR-R53U9** (significant, addressed): MCP tool result begins with `WARNINGS (n):` prefix, tool description documents the convention (AC18, AC20). `isError` stays false (write succeeded). AC18 asserts programmatic discoverability, not just substring presence.
- **RR-30U7K** (minor, addressed): `IsSoft()` placed on `*ValidationError` in `internal/metamodel/validation.go`.
- **RR-7L18Y** (significant, addressed): Lua multi-return documented as "string.gsub-style, not io.open-style". Plan section + AC25 + AC32 + RR-7L18Y reference in risk #5. Hard errors still raise via RaiseError (AC26).
- **RR-FX5GZ** (minor, addressed): scope extended to `rela.create_relation` (and `update_relation` if exists) to keep Lua surface consistent. AC23, AC24.
- **RR-61JFH** (minor, addressed): concurrency AC28 added.
- **RR-Z19ME** (minor, addressed): JSON pointer escape AC29 added.
- **RR-C7TE6** (minor, addressed): required-boolean-false regression AC30 added.
- **RR-VMDWD** (minor, addressed): `stale-review.lua` audit completed; no pcall wrapping; safe.
- **RR-3SE7A** (minor, addressed): `--strict` flag on CLI commands; AC17.
