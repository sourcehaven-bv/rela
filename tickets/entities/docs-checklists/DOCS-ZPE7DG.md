---
id: DOCS-ZPE7DG
type: docs-checklist
title: 'Docs: Clear all doclink findings and promote the rule to a blocking CI gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols documented — this change *is* doc-comment maintenance;
18 references now link correctly where they previously rendered as literal
brackets on pkg.go.dev
- [x] Non-obvious decisions carry a WHY — the upstream `failWithGuidance` doc
records why the message is shaped the way it is (a blocking check with an easy
escape hatch makes silencing the cheapest path to green)
- [x] `CLAUDE.md` updated — `doclink` marked as a gate, with a note that Go
cannot link unexported or unimported symbols at all

## Project Documentation

- [x] `CLAUDE.md` — gate/advisory split refreshed; new paragraph on reading a
finding before suppressing it
- [x] `.commentlint.yml` — gate list now `commented-code, doclink`; advisory
backlog counts refreshed (duplication 120, nil-contract 105, param-contract 5,
restatement 17)
- [x] `justfile` / `ci.yml` — `doclink` moved out of the advisory loop into
the gate; version pin bumped to v0.3.1
- [x] ~~`docs/` guides~~ (N/A: contributor tooling, not a user-facing rela
feature — same treatment as plimsoll and arch-lint)
- [x] ~~`README.md`~~ (N/A: same reasoning)

## External Documentation

- [x] Tool README documents blocking behaviour — new "Blocking runs" section
explaining that `-rank` makes a run advisory, and why the failure message is
framed fix-first
- [x] Suppression documented — unchanged and still accurate; the failure
message now points at it
- [x] ~~Migration notes~~ (N/A: consumers pin a version tag; v0.3.x is
additive apart from the backticked-span false-positive fix, which only *removes*
findings)

## Verification

- [x] Every documented command was run and produces the documented output
- [x] Counts quoted in docs match reality — `doclink` 0, and the four advisory
counts re-measured on this branch rather than copied from the previous ticket
- [x] The documented gate behaviour was tested, not assumed — a broken link
was reintroduced to confirm the gate blocks and prints the guidance
