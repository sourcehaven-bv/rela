---
id: PLAN-LKZR3B
type: planning-checklist
title: 'Planning: Named query scopes declared per entity type in schema.yaml, referenced by data-entry views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** Documented in full on TKT-EVR2TU (in/out sections). Summary: named
per-type membership predicates in schema.yaml, referenced by data-entry views,
with a `default` applying to presentation surfaces and an implicit `all` that
withdraws it.

**Acceptance Criteria:** AC1-AC11 on TKT-EVR2TU, each with a test scenario.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — two targeted codebase surveys instead of a RES entity
(the design question was "what does rela already do", not "which approach").

**Existing Solutions:**

- **Laravel/Eloquent query scopes** are the reference implementation. Named
reusable `WHERE` fragments on the model, with a *global* scope applying unless
withdrawn. Adopted: naming, the default, the withdrawal. Rejected: Eloquent's
implicit application to every consumer — rela has integrity surfaces (analyze,
validate, tracer) that must answer "what is true".
- **`internal/worlds`** (`worlds.go:5-23`) is the structural model: compile the
metamodel's declarations into a metamodel-free form at assembly, in a package
above metamodel, because metamodel may not import the engine. Copied wholesale —
the compile-at-assembly discipline, `Lookup` failing closed on an unknown name,
the zero value being usable.
- **`conditionlint.NextActionMatchers`** + `dataentry.SetNextActionMatchers`
(`nextaction.go:414-444`) is the consumer-side seam precedent: dataentry may not
import the condition engine, so the composition root bridges. Copied, including
the generic structural adapter needed to avoid the dataentry↔appbuild cycle.
- **`queryplan.ConditionPrefilters`** (`queryplan.go:261`) already lowers
request-constant equalities, including `current_user.id` and the
`is_current_user`/`has_current_user` sugar, into `store.PropPredicate`s. Reused
as-is; no new lowering was written.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `metamodel` holds raw source + validates NAMES (allowlist grammar, reserved
`all`, non-empty expression). Cannot compile: arch-lint, and `predicatefns`
already imports `metamodel`.
2. New `internal/scopes` compiles to `*predicate.Program` via
`CompileWithCurrentUser`. `mayDependOn: [metamodel, predicate, predicatefns]` —
`acl` deliberately absent.
3. `dataentryconfig` gains `query_scope:` on List and Kanban, validated to name
a declared scope.
4. **Prerequisite: TKT-VAKI0Q.** The four duplicated ACL verdict switches had to
become one before adding a third narrowing dimension — see that ticket.
5. `scopeRequest` gains `Scope`/`ScopeProps`/`ScopeEval`; `scopedHeaders`
pushes the lowerable conjuncts and applies the program as the authoritative
Go-side filter.
6. Composition root (`appbuild.QueryScopes`) compiles and bridges;
`dataentry.AdaptQueryScopes` restates the interface structurally.

**Alternative rejected:** inline `condition:` per view. Name-only means every
predicate is declared in one place, so type-level tooling is exhaustive by
construction.

**Files modified:** `internal/metamodel/{types,loader}.go`, `internal/scopes/*`
(new), `internal/dataentryconfig/{config,validate}.go`,
`internal/dataentry/{scopedread,queryscope,api_v1,app}.go`,
`internal/appbuild/queryscopes.go` (new), `cmd/rela-server/main.go`,
`.go-arch-lint.yml`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation | On invalid |
| --- | --- | --- | --- |
| scope NAME in schema.yaml | operator | allowlist regex, reserved `all`, non-empty expr | load error naming type + scope |
| scope EXPRESSION | operator | compiled by `predicate` (sandboxed, no I/O, bounded) | load error echoing the expression |
| `query_scope:` in data-entry.yaml | operator | must name a declared scope | config error listing the declared names |
| `?query_scope=` | HTTP client | must resolve; repeated param refused | request error, never a fallback |

**Security-Sensitive Operations:**

- **Scopes are UX, not access control.** The ACL read gate runs independently
and first; a scope narrows further, never wider. Pinned by the funnel's
structure — the scope filter runs on rows the verdict already permitted.
- **Fail-closed on every unknown.** An undeclared name refuses at config load
AND at request time; a nil evaluator beside a non-nil scope errors rather than
skipping the filter. Each alternative serves the UNSCOPED set.
- **`?query_scope=` cannot widen past ACL.** It selects among operator-declared
scopes; an arbitrary predicate cannot be injected.
- **Identity errors propagate.** `ErrNoCurrentUser` fails the request rather
than rendering an empty page that reads as "you have nothing".

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1/AC2 → `metamodel/queryscope_test.go`; AC3 →
`TestLookup_UnknownFailsClosed` + `TestQueryScopes_UnknownNameIsRefused` (HTTP)
+ `validate_queryscope_test.go`; AC4/AC5 → `TestQueryScopes_EndToEnd`; AC8 →
`TestScopedHeaders_ScopePropsAreASuperset`.

**Integration test:** `TestQueryScopes_EndToEnd` drives schema.yaml → real
compiler → appbuild bridge → dataentry seam → HTTP list endpoint. The unit tests
stub the evaluator; this one does not.

**Edge Cases:** empty/whitespace expression; `All` case-folded; name with
space/slash/leading digit/doubled separator; scope declared on another type;
scope on an unknown type (silent — validateLists already reports it); zero
`Compiled`; nil metamodel; repeated `?query_scope=`.

**Negative Tests:** every row of the "on invalid" column above, plus a mutation
test confirming the Go-side filter is authoritative (reverting it leaks a row
and the test names why).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** documented on TKT-EVR2TU. The surface audit turned out to be the
dominant one, and produced TKT-VAKI0Q as a prerequisite.

Effort: L (confirmed; the extraction was a separate M).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/metamodel.md` — the `query_scopes:` block
- [x] `docs/data-entry.md` — `query_scope:` on lists/kanbans, `?query_scope=`
- [ ] ~~`docs/cli-reference.md`~~ (N/A: no CLI surface)
- [ ] ~~`README.md`~~ (N/A: not a project-level change)

Both are generated from `docs-project/` — edit the source, run
`scripts/generate-docs.sh`.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** Not run as a formal `/design-review`. The design was
reviewed conversationally with the user across three decision points (naming
collision, default-scope blast radius, read-path extraction), each resolved
before implementation. Worth a `/code-review` before `done`.
