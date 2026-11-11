---
id: RR-ID8WQA6
type: review-response
title: 'Design: per-branch waiting must keep the hidden-subject rule'
finding: views clears guard verdicts when the caller cannot read the subject (RR-RKBBA7H); per-branch waiting blocks built separately would skip it.
severity: significant
resolution: 'Plan updated: each branch''s waiting block is built by the same waitingOf/addDetail path and the hidden-subject rule applies to all of them; an appbuild test covers a hidden subject with branches.'
status: addressed
---
