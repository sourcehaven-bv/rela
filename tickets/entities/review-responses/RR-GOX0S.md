---
id: RR-GOX0S
type: review-response
title: Over-clever line counting in cache-memoize test
finding: internal/dataentry/document_script_test.go:414-417. Computes 'lines' via TrimRight + Count + 1 with a zero-guard. Since ensure_newline=true on rela.write_file guarantees newline-termination, strings.Count(data, '\n') does it in one line.
severity: nit
resolution: Replaced the TrimRight+Count+1 pattern with `lines := strings.Count(string(data), "\n")`. ensure_newline guarantees trailing newline so newline-count == line-count.
status: addressed
---

From post-impl cranky review.

Fix: simplify to `lines := strings.Count(string(data), "\n")`.
