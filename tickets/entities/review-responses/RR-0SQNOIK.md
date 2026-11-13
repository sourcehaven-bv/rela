---
id: RR-0SQNOIK
type: review-response
title: An accepted benchmark slowdown fails every week
finding: Only a passing run replaces the baseline, so an intended slowdown (or an unreadable baseline, which made check exit 2) can never be cleared.
severity: significant
resolution: 'Addressed: a workflow_dispatch input reset_baseline skips the comparison so the run can become the baseline; documented in the guide and the issue text. A benchstat failure is now a report note, not an exit 2.'
status: addressed
---
