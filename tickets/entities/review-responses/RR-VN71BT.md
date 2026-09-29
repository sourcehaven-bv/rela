---
id: RR-VN71BT
type: review-response
title: Trace and analyze face lists can disclose hidden faces
finding: TraceResult.Faces and the orphan/duplicate face lists (section 5.3) are not stated to be per principal. A raw face list discloses face rows the principal cannot read and an orphan decision folded over hidden edges is an existence oracle (the gate-before-fold rule).
severity: significant
resolution: 'Amendment A4: face lists come from visibility.Resolver.Family after row gating; orphan folds only visible edges; titles use the principal''s InWorld with FaceIn; tests on CLI MCP and data-entry analyze in PR 4.'
status: addressed
---

## Finding

TraceResult.Faces and the orphan/duplicate face lists (section 5.3) are not
stated to be per principal. A raw face list discloses face rows the principal
cannot read and an orphan decision folded over hidden edges is an existence
oracle (the gate-before-fold rule).

Design: `.ignored/stage2-design.md` section 11.
