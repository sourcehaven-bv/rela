---
id: RR-CRIOYOZ
type: review-response
title: Peer existence checked before the relation verdicts
finding: Plan C step 5 answers 422 target_not_found before the relation ACL, the reverse of the BUG-K6FEVB order of the single create.
severity: minor
resolution: The handler no longer checks peer existence. The engine runs the ACL and relation checks first and treats a hidden end as missing, in the BUG-K6FEVB order.
status: addressed
---
