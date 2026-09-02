---
id: PLAN-UBRFEY
type: planning-checklist
title: 'Planning: Make searchVisibleHits fail closed when the searcher cannot redact fields'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `searchVisibleHits` degrades silently when the wired
`visibleSearcher` does not satisfy `search.FieldVisibleSearcher` — it falls back
to `SearchVisible`, which does no field redaction, **even when the active policy
hides fields**. No error, no log. GitHub issue #1093 (IB-review rela#1089).

**Scope — IN:** make the `!ok` branch fail closed, plus a regression test for
that specific path.

**Scope — OUT:**

- The `!aff.hidesAnyField()` branch. That is a **different** case and must keep
falling through: if the policy hides nothing, redaction is a provable no-op and
plain `SearchVisible` is correct. Conflating the two would break every
deployment on the Nop resolver.
- Anything about the timing exposure of redaction (that is #1094 / TKT-VR5U3Q).

**Acceptance Criteria:**

1. Policy hides fields + searcher cannot redact → the iterator yields
`search.ErrScope` and serves **no hits**.
2. Policy hides nothing + searcher cannot redact → unchanged; hits are served.
3. The error is `ErrScope` specifically, so the caller maps it to
`errACLListQuery` rather than a generic search failure.

## Research

- [x] For larger features: run `/research`
- [x] Searched for existing libraries
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — the correct shape already exists one layer down.

**Existing Solutions:** `search.Visible.SearchVisibleFields` settled this exact
question (RR-8W40EW). Its godoc:

> If a hidden func is supplied but this Visible has no provenance source, the
> method FAILS CLOSED — it yields ErrScope rather than silently returning
> un-redacted hits. A missing provenance source is a wiring bug ... and silently
> skipping redaction is exactly the oracle this closes.

So this ticket is not a design decision; it is carrying a settled principle to
the seam that was missed. Using the same sentinel (`ErrScope`) matters: the
caller in `queryservice.go` already special-cases it into `errACLListQuery`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified

**Technical Approach:** reorder the two conditions so the cheap "nothing to
redact" case returns first, then refuse when redaction is in play but the
searcher cannot do it. The error names the concrete searcher type (`%T`) so the
wiring bug is diagnosable from the message.

**Files to modify:** `internal/dataentry/helpers.go`, plus a test.

**Alternatives considered:**

1. *Log a warning and continue.* Rejected — this is the fail-open the issue is
about. A warning in a server log is not a control.
2. *Panic at wiring time.* Tempting (it is a wiring bug), but the assertion
happens per-request, not at construction, so there is no wiring-time seam to
panic from without restructuring how the searcher is held.
3. *Make `FieldVisibleSearcher` mandatory on the interface.* Rejected as bigger
than the issue: every `VisibleSearcher` implementation would have to grow the
method, including ones for which redaction is meaningless.

**Dependencies:** none new.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** none new — this changes a failure mode, not an
input path.

**Security-Sensitive Operations:** the whole ticket. The property being restored
is that a **wiring** mistake cannot silently become a **disclosure**. The
direction of the change is strictly safe: it can only refuse where it previously
served.

One detail worth stating: the error text names the searcher's Go type, not any
policy content or entity data. It is a wiring diagnostic for an operator reading
a server error, not a description of what was hidden.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

- **AC1** — a `VisibleSearcher` that deliberately does NOT implement
`FieldVisibleSearcher` (the shape a cache/metrics/tracing decorator produces),
with a resolver that reports it can hide. Assert `ErrScope`, and assert the
fallback **never ran** — the double records whether it served, because "an error
was returned" is not the same as "no un-redacted hit escaped".
- **AC2** — the same searcher with the Nop resolver: hits are served, no error.
Without this, "fail closed whenever the searcher isn't a FieldVisibleSearcher"
would also pass and would break the default deployment.
- **AC3** — `errors.Is(err, search.ErrScope)`.

**Edge Cases:** the two conditions are now independent, which is the point — the
test table covers both orders of (hides?, can-redact?).

**Negative Tests:** AC2 is the negative test.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

| Risk | Mitigation |
|---|---|
| Over-refusing breaks the Nop-resolver default | AC2 pins it; the "nothing to redact" check runs first |
| An error was returned but a hit already escaped | The test double records whether it served at all |
| The change is reverted later as "defensive" | The godoc now cites RR-8W40EW and names the oracle |

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `searchVisibleHits`'s godoc — it previously **claimed** this guarantee
("When redaction IS in play but the searcher can't do it, SearchVisibleFields
fails closed") while the code returned before ever reaching that method.
Corrected to describe what the function itself does.
- [x] ~~docs/acl-security.md~~ (N/A: the guide describes the policy model; this
is an internal wiring guard with no operator-visible configuration.)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design is
quoted verbatim from `search.Visible.SearchVisibleFields`, which settled it one
layer down. The only judgement call — keeping `hidesAnyField` as a legitimate
fall-through — is recorded under Scope.)
- [x] All critical/significant findings addressed in plan — none raised.

**Design Review Findings:** N/A.
