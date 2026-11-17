---
id: RR-QHL18DR
type: review-response
title: 'Trigger ACL check: world grants widen readers unseen'
finding: readersCovered used ReadFaces, which ignores RoleDef.Worlds and ceiling worlds/deny_worlds, so a role that reads a created record through a world could escape the reader rule.
severity: significant
resolution: 'readersCovered now fails closed: a role with any world grant that reads the created type, or a ceiling with worlds or deny_worlds that may read it, counts as a wider reader.'
status: addressed
---
