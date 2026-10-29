---
id: RR-0SERT0
type: review-response
title: WriteTarget returns only a Ref
finding: GraphReader.WriteTarget returns an entity.Ref. The plan needs the entity type (commentable check) and body (text anchor), so it must read the entity at that ref afterwards.
severity: minor
resolution: Plan step 3 reads the entity at the WriteTarget ref.
status: addressed
---

GraphReader.WriteTarget returns an entity.Ref. The plan needs the entity type
(commentable check) and body (text anchor), so it must read the entity at that
ref afterwards.
