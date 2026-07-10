---
id: RR-S4W5KI
type: review-response
title: Drop stored from_vseq/to_vseq columns; resolve endpoint versions at read time (removes oracle + NULL-lag)
finding: 'Stored from_vseq/to_vseq are a net negative (both reviewers, architect S1 + cranky S3/L1): (1) they cannot be real FKs (entity_versions PK is composite (entity_id,vseq) and from_id mutates) — only plain nullable BIGINT soft refs; (2) they are NULL for MOST freshly-created relations because the endpoint''s own create is debounced and not yet swept, undermining the ''faithful time machine'' value; (3) they introduce the TO-side oracle (RR-SDDYZO); (4) a rendering JOIN must be LEFT JOIN + NULL-render or it 500s. Fix: do NOT store the vseq pair. Store only the endpoint ids (already the composite key) and resolve ''endpoint version as-of this relation''s vseq'' at READ time via the existing lineage CTE, applying the reader''s ACL to each endpoint (natural dual-endpoint redaction per RR-SDDYZO). Removes a column pair, an oracle, and a debounce-lag footgun in one move.'
severity: significant
resolution: 'Design revised: from_vseq/to_vseq columns dropped entirely. Store only endpoint ids (already the composite); resolve ''endpoint version as-of relation vseq'' at read time via the lineage CTE with the reader''s ACL. Removes the oracle, the NULL-lag, and the column pair.'
status: addressed
---
