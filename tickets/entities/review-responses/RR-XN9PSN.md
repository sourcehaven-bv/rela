---
id: RR-XN9PSN
type: review-response
title: Pre-check skips the relation-write ACL of create.relate
finding: 'The pre-check probes only CheckCreate for a create:. CreateLinked also checks the relation ACL as the caller before writing (prepareLinkedRelation, internal/entitymanager/linked.go:135-157, 272-301). That check depends only on the source type and the caller''s global roles (CreateLinked doc, linked.go:72-76), so it can be probed with no row read. A poster who may create the entity but lacks the relation_grants permission passes the pre-check, the session moves, and the output then fails `forbidden` after the move. That is the outcome the pre-check exists to prevent. Plan change: add a relation probe for each (relate type, create type) pair to OutputWriter.Check, through a package function next to CheckCreate, and add a row to TestOutputs_PreCheckRefuses.'
severity: minor
resolution: Added entitymanager.CheckRelate, which probes RelationCreateRequest without audit and is listed in noReadGateNeeded. The pre-check calls it for each relate. Row added to TestOutputs_PreCheckRefuses.
status: addressed
---
