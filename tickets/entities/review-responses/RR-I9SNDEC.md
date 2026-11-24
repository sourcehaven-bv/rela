---
id: RR-I9SNDEC
type: review-response
title: 'Code: a re-parent clears edges of sources the caller cannot read'
finding: 'Moving an owning edge clears scoped edges of other sources found through byOwning. authorizeClears did not row-gate those sources, so a caller could delete an edge of a hidden entity, or get a 403 that confirms it exists. The noReadGateNeeded reason was false for this case.'
severity: significant
resolution: 'authorizeClears now row-gates each cleared edge''s source with requireReadableRow. A hidden or missing source refuses the write with the scope 422 and deletes nothing. The noReadGateNeeded entry is removed; the guard test passes because the gate now runs first. TestScoped_ReparentHiddenSource pins it.'
status: addressed
---
