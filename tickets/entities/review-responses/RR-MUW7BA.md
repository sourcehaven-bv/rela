---
id: RR-MUW7BA
type: review-response
title: Issue-filing logic has no test, and every failure mode is silent-green
finding: The code deciding whether up to 52 crashes get reported runs unverified once a week, and every defect found in this review failed silently at exit 0 or looked identical to a normal sweep failure. The repo gates far less consequential things (arch-lint, plimsoll, commentlint, a vendored Can I Email dataset). The parsing loop and dedup decision are pure functions of (fuzz-failures.txt, issue-list JSON), so lifting them into `scripts/fuzz-file-issues.sh` with an injectable `gh` wrapper would make them testable, reduce the workflow `run:` block to one line, and would have caught both blocking findings from the test names alone.
severity: significant
reason: Agreed in substance and it is the right end state, but extracting a script plus a bats/shell test harness is a larger change than the fix it would guard, and would land untested test infrastructure in the same PR as the behaviour change. The ad-hoc harness used here already covers the proposed cases (no trailing newline, blank row, malformed row, list-query failure, title present/absent, label failure, page cap) by extracting the real step body and stubbing `gh`; it is just not committed. Tracked as follow-up work rather than dropped — committing that harness is the actual deliverable, since the coverage already exists.
status: deferred
---

Found by cranky-code-reviewer on the TKT-8LZGME diff (finding 12).

Related gap found while verifying this ticket: **nothing in CI lints workflows
at all.** TKT-PCLGGL's verification notes claim actionlint runs via CodeQL's
"Analyze (actions)" job, but `codeql.yml` analyzes only `go` and
`javascript-typescript`. actionlint was run manually here (clean, with
shellcheck 0.11.0 so the `run:` block is genuinely inspected).

A follow-up ticket should do both: add an actionlint CI job, and commit the
step-body test harness.
