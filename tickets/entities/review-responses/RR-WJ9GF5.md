---
id: RR-WJ9GF5
type: review-response
title: git status probes pinned 200 — fixture-coupled, not route-coupled
finding: wantStatus 200 on /api/git/status and /api/v1/_git/status only holds because the fixture leaves gitOps nil; the status is environment-shaped, not a routing property.
severity: significant
resolution: Dropped both probes to any-status (0) with a comment; status pinning stays in the dedicated git handler tests.
status: addressed
---
