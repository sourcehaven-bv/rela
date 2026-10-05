---
id: RR-392FWI
type: review-response
title: Cross-entity copy writes the target without its attachment lock
finding: The copy engine locked the source id only. A cross-entity copy writes the target entity, whose attachment writers were not excluded.
severity: significant
resolution: 'lockCopyTarget locks the id the copy writes: SourceID for a same-entity copy and TargetID for a cross-entity copy.'
status: addressed
---
