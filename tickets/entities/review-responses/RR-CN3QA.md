---
id: RR-CN3QA
type: review-response
title: No TOCTOU protection on disk read
finding: The plan reads from `<sha>.json` via `os.Open`/`os.ReadFile` with no defense against the file being swapped between stat and read. Cross-process races with `persist=true` are documented as 'last writer wins' but `os.Rename` atomicity guarantees aren't spelled out.
severity: significant
resolution: 'Resolved by scope change: v1 has no disk backend, no cross-process races to defend against. When v2 adds disk, atomic rename semantics and torn-read avoidance will be explicit design points.'
status: addressed
---
