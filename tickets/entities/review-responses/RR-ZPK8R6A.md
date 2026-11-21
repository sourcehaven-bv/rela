---
id: RR-ZPK8R6A
type: review-response
title: A captured token can be replayed until it expires
finding: 'Security review: the API keeps no jti cache, so a token seen in transit can be replayed until exp.'
severity: minor
resolution: 'Kept: PUT is idempotent and GET is read-only, and a token acts on one org only. The docs ask for a short lifetime. The webhook path has the same model.'
status: addressed
---
