---
id: RR-W3RM3MU
type: review-response
title: Web resolve of a remote-deleted relation checked update, not delete
finding: Taking the remote side of a remote_deleted relation entry deletes the local relation, but the gate asked the relation update right only.
severity: significant
resolution: mayDeleteRelation asks the relation delete verdict and the remove affordance, as the DELETE route does; TestRemotes_RemoteDeletedRelationNeedsRemove.
status: addressed
---
