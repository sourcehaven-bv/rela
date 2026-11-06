---
id: RR-KDD9KGG
type: review-response
title: Skipping conflicted rows in the shared graph engine widens negated checks
finding: graphquerynaive skipped ErrConflictedRow for every query and in endpoint matching. A negated related() or an open field grant over a conflicted endpoint then read as not held. The external-ref uniqueness scan missed a conflicted holder.
severity: significant
resolution: The skip now applies only when GraphQuery.SkipUnreadable is set. Only the ACL read-grant query sets it. Endpoint matching never skips and the traversal fold clears the flag. TestGraphQuery_SkipUnreadable pins flag on and off and a negated endpoint.
status: addressed
---
