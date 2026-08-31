---
id: PLAN-QJ1TJU
type: planning-checklist
title: 'Planning: Document the ctag watermark''s cross-collection activity signal as accepted residual risk'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: prose only. Extend the `store.TypeWatermark` godoc
(`internal/store/store.go`) and the CalDAV guide
(`docs-project/entities/guides/GUIDE-caldav.md`, then `just docs`) so they state
what a type-scoped ctag discloses across differently-authorized dynamic
collections, and why each narrowing option was rejected.

OUT: every behaviour change. No touch to watermark scope, tombstone contents,
ctag computation, ACL, or the dynamic-collection resolver. If this PR changes a
single byte of program behaviour it has failed.

**Acceptance Criteria:**

1. A reader arriving at `store.TypeWatermark` finds the confidentiality analysis
without leaving the godoc. Scenario: read the godoc cold and answer "two
principals, two dynamic collections over `task`, one project each — what does
one learn about the other?" The current text cannot answer this; the new text
must, in the section itself, without following a link.
2. The godoc says why per-scope is not merely unimplemented but unavailable, and
names the concrete precondition for changing that. Scenario: a future reader who
wants to narrow the scope learns from the doc that the tombstone row is the
blocker, and that `writeEntityTombstone` is what would have to change first — so
they do not start the work before facing the GDPR question.
3. The CalDAV guide states the same limitation in operator terms, in the
`Constraints` section that already carries the other honest caveats
(TLS-is-not-ours, single-writer aliases, per-principal aliases).
4. `docs/caldav.md` is regenerated from the entity, not hand-edited. Scenario:
`just docs-check` is clean.
5. No behaviour change. Scenario: `just test` green and `git diff` touches only
comment lines, the docs entity, its generated output, and ticket entities.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — xs docs-only change, and the analysis already exists in
the ticket and in GitHub issue #1370. A research entity would duplicate them.

**Existing Solutions:**

No library question here; this is prose about this codebase's own trade-off. The
research was verification of the ticket's factual claims against the code,
because the ticket asserts things about the implementation that the doc will
then state as fact. All four were checked:

- `internal/store/store.go:585-625` — the `TypeWatermark` godoc. Confirmed it
has a "Type scope is deliberate, and over-triggers" section that argues
FUNCTIONAL safety (spurious re-sync self-corrects, a missed change strands a
client forever) and closes with "Do not 'optimize' this into a per-collection or
per-principal scope without solving the tombstone problem first." Confirmed it
never mentions confidentiality, principals-as-adversaries, or what a shared tag
discloses. The gap the ticket describes is real.
- `internal/store/pgstore/tombstone.go:13-17` — `writeEntityTombstone` executes
`INSERT INTO deletions (kind, id_a, typ) VALUES ('e', $1, $2)`. Confirmed: id
and type only, no relations, no properties. The "narrower scope cannot be
reconstructed" claim holds at the SQL level.
- `internal/dataentry/caldav_backend.go:655-676` — `watermarkCTag`. Confirmed it
hashes `"wm:%d:%s:%d"` over (len(name), name, seq) where `seq` comes from
`EntityTypeWatermark(cfg.EntityType)`. Confirmed the name-namespacing comment
says it exists so two collections do not SHARE a tag — which separates the
VALUES but leaves `seq` the only varying input, so the tags still MOVE together.
That distinction is the whole finding and is worth stating.
- `internal/dataentry/caldav_backend.go:678-702` — `collectionCTag`. Confirmed
the ETag-hashing path is the existing FALLBACK (taken when the store is not a
`TypeWatermark`, i.e. fsstore), not an alternative design one could adopt.

Prior art for the writing itself: the `watermarkCTag` godoc's own "What this
does NOT cover" section (config changes do not move the tag) is exactly this
shape — an admitted limitation with its consequence and its fix condition. The
new text should read like a sibling of it, and the CalDAV guide's `Constraints`
list is the operator-facing equivalent.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Add one section to the `TypeWatermark` godoc, after the existing "Type scope is
deliberate, and over-triggers". Placement matters: the functional argument must
be read first, because the confidentiality section's answer ("we accept it") is
only defensible once the reader knows the alternative is silent staleness.

The section must answer the three acceptance questions in order — what is
disclosed, why per-scope is unavailable, what would have to change first — and
must state the disclosure precisely enough to be SIZED rather than merely
admitted: one bit, no identity, no content, no timing precision finer than the
poll interval.

The three rejected options collapse into two paragraphs, because option 3 is not
really an alternative design: `collectionCTag`'s ETag hashing is the existing
fsstore fallback, and choosing it means deleting the watermark's reason to
exist. The godoc's "Why this exists" section already spells out that cost
(quadratic, P renders per poll), so the new text cites it rather than repeating
it — `duplication` is a commentlint rule and repeating the quadratic argument
verbatim two sections apart is precisely what it flags.

The GDPR point is the strongest argument and belongs in the godoc, not only in
the ticket: it is the reason a future implementer should stop, and it is not
derivable from the surrounding code. A reader of `tombstone.go` sees a
two-column insert and could easily think "adding a third column is cheap".

Then mirror it in the CalDAV guide's `Constraints` as one bullet in operator
terms, because the person who needs it there is deploying multi-tenant
collections, not reading store internals.

**Files to modify:**

- `internal/store/store.go` — the `TypeWatermark` godoc.
- `docs-project/entities/guides/GUIDE-caldav.md` — a `Constraints` bullet.
- `docs/caldav.md` — GENERATED, via `just docs`. Never edited by hand.
- `internal/dataentry/caldav_backend.go` — considered and rejected, see Risks.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

N/A in the usual sense — no code path is added, so no new input reaches the
program. The ticket's own subject IS an input-adjacent concern, so for
completeness: the collection name reaching `watermarkCTag` is already validated
upstream (`splitDynamicName` requires a legal entity id via `entity.ValidateID`
before the segment resolves) and the driver must be readable by the principal
(`resolveDynamic` gates on `getEntity` visibility). Neither is changed here.

**Security-Sensitive Operations:**

This ticket is a security finding, so the relevant operation is the disclosure
being documented rather than one being introduced.

- The disclosure: `EntityTypeWatermark(cfg.EntityType)` is the only varying
input to the ctag hash, so every collection over one entity type advances in
lock-step, across ACL boundaries. Confirmed against `caldav_backend.go:655-676`.
- What it is NOT: no identity, no id, no content, no count. The hash is over
`(len(name), name, seq)` and `seq` is an opaque monotonic integer; a principal
observes only that their own opaque tag differs from last poll. Timing
resolution is the client's poll interval.
- What is NOT weakened: the uniform-404 driver gate and the per-principal alias
table are untouched. This PR adds no path by which a principal can turn the
activity signal into an identity or a content disclosure.
- Error handling: no new error paths, so no new leak surface. `watermarkCTag`'s
existing errors propagate unchanged.

**Accepted residual risk** (this is the ticket's whole point): CONTROL-5-15,
severity low, accepted by the project owner. Documenting it is the mitigation of
record; the compensating controls are the ACL gate on driver lookup and the
uniform 404.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Being honest rather than inventing evidence: **a docs-only change has no
behavioural assertion to write, and adding a test here would be theatre.** There
is no new code to exercise, and a test asserting the presence of comment prose
would pin wording, not behaviour — it would fail on every future rewording and
assert nothing about the program. The verification that IS available:

- AC 1-3 (content) — editorial, verified by reading. Not machine-checkable.
- AC 4 (generated docs in sync) — `just docs-check`, a real gate that fails if
`docs/caldav.md` was hand-edited or left stale.
- AC 5 (no behaviour change) — `just test` green PLUS the stronger check that
the Go diff is comment-only, which `git diff` shows directly.
- Prose gates — `just comment-lint` (`doclink` is blocking, and the new text
will reference `writeEntityTombstone`, an UNEXPORTED symbol in a DIFFERENT
package, which Go cannot link: it must NOT be bracketed) and `just lint-md`.
- Entity integrity — `rela validate --check cardinality --check properties
--check validations` in `tickets/`.

**Edge Cases:**

Documentation edge cases, i.e. ways the text could be wrong rather than ways the
code could be:

- fsstore has no watermark, so it takes the ETag-hashing path and the signal
described does not exist there. The text must not overclaim it as universal.
- A type with exactly one collection has no cross-collection signal at all —
the finding needs two differently-authorized collections over one type.
- A type that has never had a row returns 0 (documented in the method comment),
a stable value that discloses "nothing of this type exists yet". Already covered
by the existing text; not re-litigated.
- Static collections are operator-declared, not principal-scoped, so the finding
is specifically about the DYNAMIC ones.

**Negative Tests:**

The failure mode to guard against is a doc that asserts something false, since
prose has no compiler. Every factual claim was checked against the code before
being written (see Research), and the claims that would be easiest to get wrong
are recorded there with file:line. Two were adjusted against the ticket text as
a result — see the implementation checklist.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **The doc states something the code does not do.** Highest risk in a
documentation ticket, and unlike a code bug nothing will catch it. Mitigation:
every claim verified at file:line before writing (Research section), including
the two the ticket text got slightly wrong.
- **The doc becomes a recipe.** Writing "here is how to observe activity in a
project you cannot see" more usefully than necessary. Mitigation: describe the
signal's SHAPE and BOUND (one bit, poll-interval resolution), not a procedure.
- **Padding.** The godoc is already long and this is its fifth section; a reader
who skims it learns less than one who reads a short one. Mitigation: cite the
existing "Why this exists" cost argument rather than restating it, and let the
ticket carry the long-form reasoning.
- **Editing generated `docs/` by hand.** A known repeat failure in this repo.
Mitigation: edit `docs-project/` and run `just docs`; `docs-check` is the gate.
- **Duplication across three places** (godoc, guide, ticket) drifting apart.
Mitigation: different altitudes on purpose — godoc = why the design cannot do
better, guide = one operator-facing constraint, ticket = the decision record.

**Effort:** xs. Confirmed against the ticket's own estimate; no reason to
revise.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

This ticket IS the documentation, so the usual "which docs follow the code"
question inverts.

- [x] `docs/caldav.md` — via `docs-project/entities/guides/GUIDE-caldav.md`.
One `Constraints` bullet.
- [ ] `docs/metamodel.md` — no metamodel change.
- [ ] `docs/cli-reference.md` — no command change.
- [ ] `docs/data-entry.md` — describes the `caldav:` config block; the config
surface is unchanged, and the caveat belongs with the deployment guide rather
than the key reference.
- [ ] `CLAUDE.md` — no new pattern or convention.
- [ ] `README.md` — far below project-level.

Plus `internal/store/store.go`, which is developer documentation but not a
`docs/` page.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A — no design to review. The design decision
(accept, do not narrow) was made by the project owner and is recorded in the
ticket; this plan implements the writing-down of it. The reviewable artefact is
the prose, and it is reviewed in the PR.
