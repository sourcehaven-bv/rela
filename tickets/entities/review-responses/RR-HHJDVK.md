---
id: RR-HHJDVK
type: review-response
title: parent() fallback branch untested
finding: The contain-failure branch is unreachable via resolve and had no test.
severity: minor
resolution: TestParent_FallsBackToRootOnEscape drives it with a hand-built RootedFS whose root does not contain the path.
status: addressed
---
