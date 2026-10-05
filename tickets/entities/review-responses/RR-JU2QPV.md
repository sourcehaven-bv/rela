---
id: RR-JU2QPV
type: review-response
title: Relation filter compares a face's title under another face's verdict
finding: matchRelationFilterMany read the world-preferred neighbour face and checked only the type-level face union plus 'some face readable'. With split-face conferred roles (owns->draft, reviews->published) a filter could test guesses against a draft title the principal may not read (BUG-ISJHML class). Found by both the cranky and security reviews.
severity: critical
resolution: The filter now resolves neighbours through visibleReader.servedIDsErr (ACL trims, then the world ranks) and compares the served face's title; the incoming-edge ownership check uses that face too. readableCandidates is gone. Pinned by TestFaceGateParity_RelationFilterComparesTheServedFace.
status: addressed
---
