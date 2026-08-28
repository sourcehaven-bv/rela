---
id: PLAN-SKXUEV
type: planning-checklist
title: 'Planning: Adopt commentlint in CI: comment-discipline gate + advisory report'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Note on sequencing.** This ticket was written *after* the work, which was
> done exploratorily (evaluate the linter → find its rules wanting → improve
> the tool → adopt it). The checklist below records what was actually decided
> and verified, not a plan written in advance. Flagged rather than backdated.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: commentlint as a CI job (gate + advisory report), `.commentlint.yml` with
rule rationale, justfile recipes, `CLAUDE.md` docs, review-checklist guidance on
suppression, and the handful of genuine comment fixes found while wiring it.

OUT: working down the advisory backlog (duplication 119, nil-contract 100,
doclink 58, param-contract 5, restatement 19). Each is its own follow-up. Also
out: the upstream tool itself, which lives in its own repo and is already
released at v0.2.0.

**Acceptance Criteria:**

1. `just comment-lint` exits 0 on develop — verified, "no findings across 9876
comments".
2. The gate fails on a regression — verified by construction: `commented-code`
is 0 today, and the rule has a unit test upstream.
3. `just comment-report [rule]` lists advisory findings and never fails —
verified for `param-contract`.
4. CI job is valid YAML and appears in the job list — verified by parsing
`ci.yml` (`comment-lint`, 5 steps).
5. A false positive can be suppressed two ways — verified end-to-end upstream
(inline directive and `allow-phrases`), and used in anger here for
`exifHeaderLen`.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: `s` effort, single CI job
following an established in-repo pattern)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — small change, prior art already in-repo.

**Existing Solutions:**

Existing tooling was tested directly against a deliberately broken doc link
(`[Set.Enforce]` where the method is `EnforceUpdate`):

- `go vet` — exit 0
- `staticcheck` — 0 issues
- golangci-lint **`godoclint`** — 0 issues (enabled specifically to test this)
- `go doc` — renders the link as plain text, no diagnostic

Go's `go/doc/comment` degrades an unresolvable link silently by design: a
`DocLink` is only produced when the caller supplies a `LookupSym` resolver.
Nothing in the standard pipeline reports the failure, which is why the `doclink`
rule is worth having at all.

Codebase prior art: `arch-lint` (`justfile:210`) and `plimsoll` (`justfile:218`,
CI job "God-object lint", `ci.yml:153`) are the same shape — a pinned
third-party linter, version in the justfile, mirrored in `ci.yml`. This ticket
copies that shape deliberately rather than inventing a new one.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Mirror the plimsoll job exactly: `go install …/commentlint@v0.2.0`, pinned
version duplicated in the justfile with a "keep in sync" comment (the existing
convention). Two CI steps rather than one:

- gate: `commentlint -rules commented-code ./internal ./cmd`
- report: `commentlint -rank -top 30 ./internal ./cmd`, `continue-on-error`

**Alternatives rejected:**

1. *Gate everything.* Would be red on arrival with 301 findings. A gate that
fails on day one trains people to ignore the job, so the split exists.
2. *Report only, no gate.* Nothing would catch a regression;
`commented-code` is clean and cheap to keep clean.
3. *Vendor the linter into the repo.* Rejected — it is generic, useful
outside rela, and the repo already consumes plimsoll this way.
4. *Enable `too-long` / `scope-reach`.* Rejected on evidence; see the ticket
body and `.commentlint.yml`.

**Files to modify:** `.commentlint.yml` (new), `.github/workflows/ci.yml`,
`justfile`, `CLAUDE.md`, `tickets/templates/entities/review-checklist.md`, plus
5 comment-only source edits.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The linter reads Go source and `.commentlint.yml`, both operator-authored and
already in the repo. No runtime input, no network, no untrusted data. It runs in
CI only and produces no artifact.

The one real consideration is **supply chain**: CI installs a third-party
binary. Mitigated the same way plimsoll is — pinned to an immutable version tag
(`@v0.2.0`), from an org-owned public repo, resolved through the Go module proxy
with checksum verification. Not a new class of exposure.

**Security-Sensitive Operations:**

None introduced. One *improvement*: the `credentialFileMode` fix documents why
the git credentials file (which holds an access token in cleartext) must stay
`0600` — the previous comment restated the constant and hid the reason.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

Rule behaviour is tested upstream (20 unit tests in the commentlint repo,
covering config parsing, glob matching, inline directives, and each rule). In
*this* repo the integration surface is what gets verified: `just comment-lint`
exits 0, `just comment-report` runs and never fails, `ci.yml` parses and
registers the job, `golangci-lint` and the full test suite stay green over the
comment edits.

**Edge Cases:**

- UTF-8: excerpt truncation must not split a multi-byte rune. This corpus is
full of em dashes and arrows; byte-slicing produced invalid UTF-8 and broke a
consumer. Fixed upstream (rune-aware truncation) before adoption.
- Empty/absent config: the tool must work with no `.commentlint.yml`.
- A comment already carrying a `Nil:` tag must not re-fire (convergence).
- Suppression must be narrow: naming one rule must not silence others.

**Negative Tests:**

Malformed `.commentlint.yml` (bad bool, non-numeric setting, orphan list item)
must fail loudly with a line number, not silently ignore the file. Covered
upstream by `TestParseFileConfigErrors`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl) — `s`

**Risks:**

1. *False positives erode trust.* Highest risk, since every rule is a
heuristic over prose. Mitigated three ways: only a clean rule gates; two
suppression mechanisms with a required reason; and the rules were tuned against
this corpus before adoption (e.g. `nil-contract` dropped from 196 to 100 by
excluding descriptive `non-nil` and error-only returns).
2. *Backlog never worked down.* Advisory findings can be ignored forever.
Mitigated by recording the count per rule in `.commentlint.yml` and the ticket,
so drift is visible, and by the review-checklist rule that a finding your own
diff introduces should be fixed or suppressed.
3. *Version drift between justfile and ci.yml.* Same risk plimsoll already
carries; mitigated the same way, with a "keep in sync" comment on both.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `CLAUDE.md` — new tooling, documented next to plimsoll/arch-lint
- [x] `tickets/templates/entities/review-checklist.md` — PR-checklist guidance
- [x] `.commentlint.yml` — inline rationale for every disabled rule
- [x] N/A for `docs/` — this is contributor tooling, not a user-facing feature;
no CLI command, no API, no UI surface

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design
question — which rules gate vs. report, and on what evidence — was settled
empirically against the corpus during the work, and is recorded in the ticket
body and `.commentlint.yml`. See the sequencing note above.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
