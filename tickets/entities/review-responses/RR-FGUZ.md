---
id: RR-FGUZ
type: review-response
title: Write-path Visible probe must run BEFORE body parse / IsLocked / If-Match
finding: 'Plan says ''at the top of the handler, after entity lookup.'' Today''s handleV1UpdateEntity flow: lookup → IsLocked 422 → If-Match 412 → body parse 400 → validation. If Visible runs after body parse, a malformed body to a hidden entity returns 400 = existence oracle (''URL is valid but body is wrong'' = ''this entity exists''). If after If-Match, the 412 path is an oracle. If after IsLocked, the 422 path is. Pin: Visible runs immediately after the existing 404 check at line ~762 — BEFORE IsLocked, BEFORE If-Match, BEFORE body parse. Same in handleV1DeleteEntity (line ~944) and handleV1EntityAction (line ~1335). Explicit AC test: PATCH on hidden-id with malformed JSON body returns 404, not 400.'
severity: significant
resolution: Gate moved to top of handler — before getEntity, IsLocked, If-Match, body parse — in handleV1UpdateEntity, DeleteEntity, CloneEntity. handleV1EntityAction routes through handleV1CloneEntity. Test TestACLWrite_PatchOnHiddenIs404 has 4 cases (valid body, malformed body, stale If-Match, current If-Match) all asserting 404.
status: addressed
---
