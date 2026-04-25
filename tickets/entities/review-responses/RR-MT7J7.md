---
id: RR-MT7J7
type: review-response
title: t.Skip() turns regression net into silent no-op when zero matches
finding: loader_test.go uses t.Skip() if the walk finds zero metamodel.yaml files. If repoRoot is ever computed wrong (e.g., the package gets refactored and cwd, "..", ".." no longer points at the repo root), or if the walk filter becomes overly aggressive, the test passes silently with a green check instead of failing. A regression net that can disappear silently isn't a net. Replace t.Skip with t.Fatalf on zero matches — or anchor the search by walking up to find go.mod instead of dead-reckoning ../.. .
severity: significant
resolution: Replaced t.Skip on zero matches with t.Fatalf. Added findRepoRoot helper that walks up looking for go.mod instead of dead-reckoning ../.. — fails the test if go.mod is never found, so the regression net cannot silently disappear if the package moves.
status: addressed
---
