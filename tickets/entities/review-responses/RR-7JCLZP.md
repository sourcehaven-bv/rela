---
id: RR-7JCLZP
type: review-response
title: Batched reads are new construction sites for the AllowAll + FaceIn fail-open shape
finding: 'pushdown.go:83-95 documents that the AllowAll branch never builds a GraphQuery, so FaceIn must be carried on the EntityQuery or the most privileged principals get no face narrowing. Every batched read in D.1 (ListRelations{EntityIDs}, ListEntityHeaders{IDs}, computeFaces, section columns) can forget it, and the failure is silent: a manager sees a draft title under the published world. AC6 covers hidden neighbours but not this shape.'
severity: significant
resolution: 'AC6 extended with an AllowAll-under-published-world test: 50-row list with relation columns and a section view, asserting no draft-face title appears through any batched path. Batched EntityQuery/RelationQuery values are built by one helper that stamps World/FaceIn/FromFace from the request, so no call site constructs them by hand.'
status: addressed
---
