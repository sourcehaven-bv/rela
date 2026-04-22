---
id: RR-PW4AO
type: review-response
title: sortStrings is hand-rolled insertion sort; sort.Strings exists
finding: Pre-existing hand-rolled sort in fsstore.go. Would be O(n^2) on large inserts.
severity: nit
reason: Pre-existing code, not touched by this PR. Unrelated to path-safety migration. File as a separate cleanup ticket if it ever matters (N is small in practice — number of entity files in a project).
status: wont-fix
---
