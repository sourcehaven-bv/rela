---
id: PLAN-ZNZ36L
type: planning-checklist
title: 'Planning: Document why rela import bypasses transition guards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: a godoc on `importEntity` recording why it writes to the store directly, why
that is not a boundary, and why it diverges from sync.

OUT: any change to the importer's write path. OUT: revisiting RR-NB135 — sync's
decision stands on its own reasoning, and the point of this ticket is that the
two paths differ for a REASON rather than by accident.

**Acceptance Criteria:**

1. A reader at `importEntity` learns why the guards are absent without leaving
the file.
2. The comment states what would CHANGE the answer — a non-CLI caller — so the
decision is revisitable rather than inherited.
3. The claims in it are true. Specifically: the importer has exactly one
non-test caller, and `normalize` cannot change status.

AC3 is not ceremony. The comment asserts facts about OTHER files, and a comment
that is confidently wrong about its neighbours is worse than no comment — it
gets trusted.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — a comment.

**Existing Solutions:**

The governing prior art is RR-NB135, which decided the SYNC path enforces
transition guards. This ticket deliberately diverges, so the comment has to say
why rather than leave the inconsistency for a reviewer to find.

Verified against the code rather than assumed:

- `importer.New` has exactly ONE non-test caller: `internal/cli/import.go:28`.
- `rela create` (`internal/cli/create.go:54`) and `rela restore`
(`internal/cli/restore.go:61,68`) go through **EntityManager** and ARE guarded.
- `rela normalize` (`internal/cli/normalize.go:55`) writes to the store directly
but only rewrites markdown headers — grep for `status` in that file returns
nothing.

That last set matters because the intuitive summary — "the CLI writes directly
anyway" — is FALSE, and a decision resting on it would be sloppy. Import is a
single deliberate exception, not a general posture.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** extend the godoc on `importEntity`. Three reasons
weighed TOGETHER rather than listed, because each alone is weaker than the set:
import loads states that already exist; the importer is CLI-only; a guard is a
speed bump against someone who already has store access.

**Alternatives considered:**

- *Enforce, matching sync.* Rejected: it would reject a legitimate
`status: done` record from the system being migrated from — exactly the data an
import carries. The guard would not prevent bad states, it would prevent true
ones.
- *Enforce with a `--skip-guards` flag.* Rejected as ceremony around a
non-boundary: the flag would document itself as bypassable, and the operator it
"protects" from can already edit the store directly.

**Files to modify:** `internal/importer/importer.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** unchanged — this adds a comment. The importer
still validates against the metamodel (`ValidateRelation`, and the entity def
lookup that supplies default status).

**Security-Sensitive Operations:** the decision itself is the security-relevant
artefact. Recording it means the next reviewer evaluates the REASONING rather
than rediscovering the absence, which is what happened here.

Worth stating plainly: this is not a claim that unguarded import is harmless in
general. It is a claim that the guard defends nothing against the only actor who
can reach this code. The comment names the condition that would break that — a
non-CLI caller — so the argument fails loudly rather than quietly.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** none — there is no behaviour to test. Adding a test that
asserts "import bypasses guards" would pin the CURRENT design as if it were a
requirement, making the deliberate exception harder to revisit than the code it
documents. That is the opposite of the ticket's intent.

What IS verified, by grep rather than by test: the two factual claims under AC3.
Recorded in the implementation checklist with the commands used.

**Edge Cases / Negative Tests:** N/A for a comment.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *The comment is wrong about its neighbours.* The real risk here — it asserts
facts about `cli/create.go`, `cli/restore.go` and `cli/normalize.go`, and a
confidently-wrong comment gets trusted. Mitigated by verifying each claim
against the code and recording the commands.
- *The decision is inherited rather than revisited.* Mitigated by naming the
condition that invalidates it (a non-CLI caller) inside the comment itself.

**Effort:** xs

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A for `docs/` — no user-visible behaviour changes, and `rela import`'s
CLI documentation describes what it does rather than which internal write path
it takes. The audience for this decision is a maintainer or a security reviewer,
which is why it belongs in the godoc.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none. The one thing worth flagging is that the
divergence from RR-NB135 is deliberate and must READ as deliberate — an
undocumented inconsistency between two write paths is exactly what invites the
next review finding.
