---
id: RR-NTKQVY
type: review-response
title: 'PR-A: scanRelation FromFace tether + FALSE predicate readability'
finding: pgstore's scanRelation never populates FromFace — correct while all rows are default-tailed, but with the storetest States suite gated off for pg, nothing would catch PR-B forgetting the scan when the column lands (every edge would read default-tailed while the DB says otherwise; MatchRelation equality would silently return wrong rows). Separately, relationWhere's FALSE predicate reads as if it might interact with the numbered add() placeholders.
severity: significant
resolution: TODO(TKT-DOFYR1-PR-B) marker with the failure-mode explanation added at scanRelation; comment on the FALSE predicate noting it carries no placeholder so ordering is irrelevant. PR-B's checklist (in the plan) already includes populating the scan and un-gating the suite.
status: addressed
---
