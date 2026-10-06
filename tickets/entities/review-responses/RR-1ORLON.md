---
id: RR-1ORLON
type: review-response
title: CLI renumber writes the default tail
finding: rela renumber passed RelationOptions without FromFace, so a content edge on a faced source was addressed at the zero tail.
severity: significant
resolution: RelationOptions carries p.rel.FromFace. Pinned by TestRenumber_FacedTails.
status: addressed
---
