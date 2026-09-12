---
id: REV-PET81J
type: review-checklist
title: 'Review: Replace CodeQL with Semgrep frontend DOM-XSS rules'
status: done
---

## Automated Checks

- [x] All tests pass — no Go or TypeScript behaviour changed; the only source
edit is a comment. `just ci` run locally, see the PR section below
- [x] Lint clean — `.golangci.yml` change is comment-only; ticket markdown is
outside markdownlint's globs (`tickets/**` is excluded)
- [x] Coverage maintained — no executable code added or removed, so package
floors and the frontend ratchet are unaffected

## Code Review

- [x] ~~Run `/code-review`~~ (N/A: no application code — two workflow files, a
rule file, and a comment. Reviewed directly against the coverage question below,
which is the actual risk in this change)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

### Coverage analysis: what is lost and what is gained

This is a replacement, so the honest question is what CodeQL caught that Semgrep
will not.

**Lost.** CodeQL ran two languages. The `go` matrix leg is the real reduction:
it does interprocedural taint tracking that Semgrep's pattern rules do not
attempt, and it produced genuine findings historically (RR-1TCU6X,
`go/clear-text-logging`; TKT-R8QEV3's `go/path-injection` alerts). That loss is
partly offset by gosec's taint checks, which are now ENABLED in `.golangci.yml`
— G702 through G706, including path traversal, SSRF, XSS and log injection.
When CodeQL was introduced those five were all excluded, so the Go-side gap is
much smaller today than the raw "CodeQL is gone" framing suggests. It is not
zero: gosec's analysis is shallower than CodeQL's.

**Gained.** The `javascript-typescript` leg is replaced by rules that actually
fail the build. CodeQL results land in the Security tab; these rules run with
`--error`, so a new DOM-XSS sink blocks the PR. The rules are also readable and
editable in-repo rather than being a black-box query pack.

The user's decision is to ship the swap as-is. Recorded here so the Go-side
tradeoff is visible to whoever revisits it, not as an objection.

### Rule quality

Reviewed for the failure mode that matters in a security check — a rule that
never fires. The registry packs were confirmed to contain no browser DOM-XSS
rules by canary before these were written, and the rules here were confirmed to
fire on all six sinks and to stay silent on the six correct-usage cases. See
IMPL-G0HGN7 for the evidence.

The single existing finding was annotated rather than suppressed by weakening
the rule, which keeps the rule able to catch a misconfigured sanitizer later.

## Acceptance Verification

- [x] Each acceptance criterion tested — see the evidence block in IMPL-G0HGN7
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Semgrep job runs and is not a no-op — PASS (6 rules, 221 targets, ~100% parse)
- Rules fire on real DOM-XSS sinks — PASS (9 findings across all six rules on a
canary)
- Rules do not produce a wall of false positives — PASS (0 findings on the
repo after one true-positive site was annotated; literal and
already-sanitized forms correctly do not fire)
- Deleting codeql.yml does not strand a required check — PASS (see below)
- No dangling CodeQL references left in config — PASS (`grep` finds none outside
`tickets/`, which are historical records)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — a finding prints the rule id and the
remediation, including how to annotate a provably-safe site

## Branch protection

`CodeQL` is **not** a required status check on the `develop` ruleset, so this
deletion does not leave the merge queue waiting on a check that can never
report. Required checks are: Test, Lint, Lint Markdown, Fuzz, Docs, Rela
Tickets, E2E, Architecture, God-object lint, Postgres Backend, Frontend, Build,
Demos, and the six Cross-Compile jobs.

`Semgrep` is likewise not in that list. The job will run and can fail a PR, but
it cannot block a merge until someone adds it to the ruleset — a follow-up once
it has reported green on develop a few times.

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1569
