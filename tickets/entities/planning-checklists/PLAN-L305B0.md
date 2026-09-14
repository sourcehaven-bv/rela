---
id: PLAN-L305B0
type: planning-checklist
title: 'Planning: Fuzz sweep files one issue per failing target'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: the issue-filing step of `.github/workflows/fuzz-sweep.yml` — one issue per
failing `(package, target)` pair instead of one issue per sweep, with dedup
matched on that pair.

OUT: `scripts/fuzz-all.sh` and the `fuzz-failures.txt` format (unchanged — the
summary already carries one row per failing target, which is exactly the input
this needs); the artifact upload; the `fuzz-failure` label; the per-PR smoke
job; triage of the findings already accumulated in #993; adding an actionlint
CI job (noted as a follow-up).

**Acceptance Criteria:**

1. A sweep failing N targets files N issues, one per target.
   Test: feed a 3-row summary with `gh` stubbed, assert 3 creates.
2. A target failing again while its issue is open gets a comment there, not a
   duplicate issue.
   Test: stub reports an open issue for one title, assert comment-not-create.
3. The same target name in different packages does not alias onto one issue.
   Test: 3-row summary, same target, three backends, assert 3 distinct creates.
4. A setup error (script exit 2, no summary file) files nothing.
   Test: run with no `fuzz-failures.txt`, assert no gh calls and exit 0.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: xs chore, single-file change)
- [x] ~~Searched for existing libraries~~ (N/A: no library involved; this is
  workflow shell + `gh`)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the
  behaviour is specified by the issue, not by prior art)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (xs chore)

**Existing Solutions:**

- TKT-PCLGGL built the sweep and chose label-based dedup deliberately
  ("instead of creating a new one each week"). This ticket supersedes that one
  decision; the rest of the design stands.
- `scripts/fuzz-all.sh:72` already writes one `pkg target [kind]` row per
  failing target, so per-target identity needed no script change — only the
  consumer had to stop flattening it.
- Checked the other workflows for a standalone `jq` dependency: none use it,
  which informed keeping the lookup on `gh --jq`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Loop over `fuzz-failures.txt` rows (`sed` strips the `[kind]` brackets so
`read -r pkg target kind` splits cleanly). Per row, build a title
`Fuzz failure: <Target> (<package>)`, write a body naming that one failure, and
look for an open `fuzz-failure` issue whose title matches exactly — comment if
found, create if not.

Alternatives rejected:

- *Target name alone as identity* — `FuzzPropertyValuesTypeZoo` fails
  independently on fsstore, memstore and sqlitestore; those are separate
  backend bugs and must not share an issue.
- *Standalone `jq` for the exact match* — works (preinstalled on
  `ubuntu-latest`) but adds a dependency no other workflow here relies on.
  `gh --jq` with `env.TITLE` does the same job.
- *`gh issue list --search`* — GitHub's search is fuzzy and would alias
  near-identical titles onto one issue.

**Files to modify:**

- `.github/workflows/fuzz-sweep.yml` (issue-filing step + header comment)
- `tickets/entities/tickets/TKT-PCLGGL.md` (note the superseded decision)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

`fuzz-failures.txt` — produced by `scripts/fuzz-all.sh` from Go package paths
and `Fuzz*` function names discovered in-repo. Not attacker-controlled in any
normal flow, but it is interpolated into a shell variable and a jq filter, so
it is treated as untrusted: the title is passed to `gh --jq` through the
**environment** (`env.TITLE`) rather than spliced into the filter string, so a
name containing quotes cannot break out. All expansions are quoted; rows with
an empty target are skipped.

**Security-Sensitive Operations:**

- `GH_TOKEN` — the step already runs with `issues: write`; the change does not
  widen permissions and the token is never echoed.
- Issue bodies contain only package paths, target names, kind and the run URL —
  no corpus bytes, so a crashing input cannot leak into a public issue.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

The step's `run:` body is extracted from the parsed YAML and executed with `gh`
stubbed on `PATH`, so the real shell logic runs rather than a paraphrase of it.
Each acceptance criterion above maps to one such run.

**Edge Cases:**

- Same target, three different packages (must produce three issues).
- `error` kind vs `fuzz-crash` kind (body must explain the right one).
- Empty/blank row in the summary (skipped).
- Missing summary file entirely (setup error — file nothing, exit 0).
- A title containing shell/jq metacharacters (handled via `env.TITLE`).

**Negative Tests:**

- `gh` fails mid-loop: later targets must still file, the step must exit
  non-zero, and the failure must be named in the log.
- Non-matching title must return empty, not the first open issue.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Cannot be tested by CI before it runs for real* — the step only executes on
  a failing weekly sweep. Mitigated by extracting and running the actual step
  body against stubs, and by actionlint+shellcheck over the block. Residual
  risk accepted: first real exercise is the next sweep.
- *Issue burst* — the first sweep after this lands files one issue per
  currently-failing target (~3 based on the last run). Intended behaviour;
  flagged to the user.
- *#993 goes quiet* — no new title matches it, so it stops receiving comments
  while still holding unfixed findings. Out of scope here; raised for triage.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal CI change, no user-facing
  surface)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A:
  `kind=chore`, not enhancement/docs)

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed

The workflow's own header comment is updated in-place, and TKT-PCLGGL's
approach note records the superseded decision.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: xs chore,
  single-step workflow change; reviewed inline with the user)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None (no design review run — see above)
