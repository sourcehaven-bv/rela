---
id: RR-OU5MTWI
type: review-response
title: 'Code: clear authorization cached per relation and source type'
finding: 'authorizeClears reused one ACL verdict per (relation type, source type). Grants that depend on the row (role_relations, when:) could let one allowed source authorize the clear of another.'
severity: significant
resolution: 'Each cleared edge is authorized on its own source; the cache is gone. TestScoped_ClearAuthorizedPerSource pins it.'
status: addressed
---
