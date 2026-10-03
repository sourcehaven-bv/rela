---
id: RR-A99RJ1
type: review-response
title: storetest lacks per-face lineage and named-face HighestID
finding: No test that ListVersions(Ref) of one face excludes other faces across rename and id reuse; no HighestID test for a family stored only at named faces.
severity: significant
resolution: Added storetest FaceLineage (RenameAndReuseStayPerFace and UnaddressableRefHasNoHistory). HighestID over named-faces-only families is already covered by storetest states.go HighestIDSeesFacedEntities.
status: addressed
---
