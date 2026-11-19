---
id: RR-JF0SWFV
type: review-response
title: Requiring every face readable is an oracle
finding: A 404 on /_changes for face A when sibling face B is hidden reveals that B exists.
severity: significant
resolution: 'Plan: audit.Subject gains Face (the entity''s face; on a relation, its From face) and audit_subjects stores it. /_changes resolves one face as history does and reads only that face''s rows plus faceless relation-To rows. A byte-identical test with B hidden, absent and present.'
status: addressed
---

A 404 on /_changes for face A when sibling face B is hidden reveals that B
exists.
