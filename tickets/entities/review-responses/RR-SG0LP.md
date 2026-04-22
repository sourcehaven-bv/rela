---
id: RR-SG0LP
type: review-response
title: document-live-update tests should be ported as test.skip, not dropped
finding: Those tests encode intended behavior. Porting with test.skip preserved + TODO costs nothing and keeps the intent recorded.
severity: minor
resolution: 'Initially ported as empty test.skip stubs to preserve intent; on follow-up review the stubs were judged to be TODOs in test clothing (bodies contained only comments, nothing to unskip). Deleted the file and filed TKT-0K5YH to track the actual work of adding documents: coverage when the inline project grows one.'
status: addressed
---
