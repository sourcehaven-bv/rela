---
id: RR-LDCFCHI
type: review-response
title: Audit attribution claim depends on principal_property
finding: If the lookup matched a peer name the audit would show an entity id. Security review.
severity: nit
resolution: Resolved by skipping the lookup for peers; the docs say a peer's name is never replaced.
status: addressed
---
