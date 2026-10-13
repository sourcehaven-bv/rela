---
id: RR-C00RAC
type: review-response
title: migrate() gets no face, so a step cannot pick its own edges
finding: forEachEntity hands every face row, but migrate(e) had no face field; with reads a script cannot filter get_relations by from_face or address its own row.
severity: significant
resolution: entityToTable now sets face; the comment explains why it is no longer inert. TestLuaReads_StepFaceAndFaces checks per-face edges.
status: addressed
---
