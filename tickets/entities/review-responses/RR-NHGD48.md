---
id: RR-NHGD48
type: review-response
title: Relation history version read ran the gate with an empty type
finding: serveRelationHistoryVersion passed storedType(snap.From) to vr.ref even when the source was gone and the type was empty.
severity: minor
resolution: The meta read now runs only when the stored type is non-empty. TestRelationHistory_GoneSourceServesNoMeta covers the gone-source path.
status: addressed
---
