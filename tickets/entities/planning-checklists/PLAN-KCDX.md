---
id: PLAN-KCDX
type: planning-checklist
title: 'Planning: ACL v0 PR 3: Wire acl.yaml into appbuild + non-loopback warning + docs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** PR 3 of the v0 trilogy. Inherits from PLAN-ZDL4K. Specifically:

- `internal/appbuild/appbuild.go` — load `acl.yaml` at project root via `acl.LoadPolicy`; fall back to `acl.NopACL{}` on `os.ErrNotExist`; `--read-only` (via `WithACL(ReadOnlyACL{})` from PR 1) wins over both.
- `internal/appbuild/appbuild_acl_test.go` — wiring tests.
- `cmd/rela-server/main.go` — non-loopback + missing-acl-yaml → one `slog.Warn`.
- `docs/security.md` — ACL section.
- `docs/audit-log.md` — `denied-write` op.
- `CLAUDE.md` — brief note about the ACL package.

**Out:** anything not on the PR 3 AC list (no schema changes, no Lua, no MCP
intersection).

**Acceptance Criteria** (cherry-picked from PLAN-ZDL4K):

1. **AC3.1** — `appbuild.Discover` loads `acl.yaml` from project root and passes the resulting Declarative (or NopACL on absence) into `entitymanager.Deps`.
2. **AC3.2** — `--read-only` continues to win over `acl.yaml` (verify via test).
3. **AC3.3** — `rela-server --bind 0.0.0.0` without `acl.yaml` (and without `--read-only`) emits one `slog.Warn`. Loopback bind silent. `acl.yaml` present → no warning.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

Pattern: `internal/audit/filesystem.go` is the model — `appbuild` constructs the
production sink in `Discover`, falls back to a safer default. `acl.LoadPolicy`
already returns `os.ErrNotExist` for that purpose (PR 2).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### appbuild change

```go
func loadACL(paths *project.Context) acl.ACL {
    policy, err := acl.LoadPolicy(filepath.Join(paths.Root, "acl.yaml"))
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return acl.NopACL{}
        }
        slog.Warn("acl: failed to load acl.yaml; falling back to NopACL", "error", err)
        return acl.NopACL{}
    }
    return acl.NewDeclarative(policy)
}
```

Wire into `New`: the default `o.acl` becomes `loadACL(paths)` instead of
`acl.NopACL{}`. **`WithACL` still wins** (set via opts after defaulting) —
preserves PR 1's `--read-only` behavior.

### rela-server warning

In `cmd/rela-server/main.go`, after `appbuild.Discover` returns and after the
existing non-loopback warning block, add:

```go
if !isLoopbackHost(f.bind) && !f.readOnly {
    // Detect whether the loaded ACL is NopACL — if so, warn.
    if isNopACL(svc.ACL()) {  // requires Services.ACL() accessor
        slog.Warn("rela-server bound beyond loopback without acl.yaml: anyone reaching this server can write; see docs/security.md")
    }
}
```

Wait — adding `Services.ACL()` accessor pulls `acl` into `appbuild.Services`
API. Simpler: track whether `loadACL` returned `NopACL` via a second `Discover`
return value or a new `Services` method. Let me use a method.

### Files

| File | Change |
|---|---|
| `internal/appbuild/appbuild.go` | Add `loadACL(paths)`; default `o.acl` from it; add `Services.ACL() acl.ACL` accessor |
| `internal/appbuild/appbuild_acl_test.go` | NEW — three tests: acl.yaml present → Declarative; absent → NopACL; WithACL overrides both |
| `cmd/rela-server/main.go` | After Discover, emit `slog.Warn` when non-loopback + NopACL + !read-only |
| `docs/security.md` | ACL section: schema example, delegate-X explanation, trust model, --read-only |
| `docs/audit-log.md` | One paragraph on the `denied-write` op |
| `CLAUDE.md` | Brief paragraph: ACL package location, consumer-side interface rule applied to it, "Lua never on read path" discipline |

**Alternatives:**
- *Discover returns a tagged result* (e.g. `(*Services, bool, error)` where bool is "ACL was loaded from file") — rejected, breaks Discover's API for one call site.
- *Probe the file in main.go directly* — duplicates path logic; rejected.

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

`acl.yaml` content: trusted (PR-reviewed). Loader already validates in PR 2.
Warning logs use operator-facing strings only; no policy content leaks.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

| AC | Test |
|---|---|
| AC3.1 | `TestDiscover_ACLPresent_LoadsDeclarative` — write a temp acl.yaml, call Discover, assert `Services.ACL()` is `*Declarative` |
| AC3.1 | `TestDiscover_ACLMissing_UsesNop` — no file, assert `Services.ACL()` is `NopACL{}` |
| AC3.2 | `TestWithACL_OverridesLoadedPolicy` — write acl.yaml + pass `WithACL(ReadOnlyACL{})`, assert `Services.ACL()` is `ReadOnlyACL{}` |
| AC3.3 | Manual / smoke — `rela-server` `main_acl_test.go` is non-trivial; alternative: helper `shouldWarnNoACL` tested directly. Decision: ship a small `shouldWarnNoACL(bind string, isReadOnly bool, hasPolicy bool) bool` helper in main.go with a unit test. |

**Edge cases:** acl.yaml present but unreadable (perm denied) → warn + NopACL
fallback (logged but not fatal); acl.yaml empty → empty Policy, allow-all
default (no roles defined → every write denied... wait, that's wrong, need to
think). Empty Policy with no `default` role → every write denied.
Operator-confusing. Mitigation: document this in docs/security.md.

**Negative tests:** appbuild_acl_test covers malformed yaml propagating an error
(already covered indirectly via LoadPolicy tests in PR 2, but a top-level
"Discover surfaces parse errors gracefully" test is worth one assertion).

## Risk Assessment

- [x] Technical risks
- [x] Security risks
- [x] Effort `s` (≈1 day)

| Risk | Severity | Mitigation |
|---|---|---|
| Adding `Services.ACL()` accessor leaks acl into appbuild's API surface | Low | Already imports acl per PR 1; one more method is fine. |
| `slog.Warn` on non-loopback fires for legitimate behind-proxy setups | Low | Document in docs/security.md; warning text says "see docs/security.md" |
| Empty acl.yaml = "deny everything" semantic surprise | Medium | Document explicitly: "an `acl.yaml` with no `default` role denies every write to every principal". Test pins the behavior. |

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

`docs/security.md` ACL section is the main user-facing doc. CLAUDE.md note is
for code authors. `docs/audit-log.md` add is one paragraph.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design fully covered by PLAN-ZDL4K + PR 1 + PR 2)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)
