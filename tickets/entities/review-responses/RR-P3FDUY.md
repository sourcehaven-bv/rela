---
id: RR-P3FDUY
type: review-response
title: 'Design: deleted-row gate must match the HTTP history rule'
finding: Gating deleted rows on history:read alone would let a script list deleted faces whose face grant the caller lacks, or entities whose live siblings it cannot read.
severity: significant
resolution: 'Plan mirrors resolveHistorySubject: history:read, FaceAllowed on the face, and a readable same-type live sibling when the id still has live rows. Unit tests per condition.'
status: addressed
---
