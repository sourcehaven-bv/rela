---
id: RR-3VRCLV
type: review-response
title: Converter duplicates CreateLinked
finding: The converter re-implements the create-plus-edge pipeline (relation prep, audit order, createCore retry) beside entitymanager.CreateLinked. CreateLinked lacks an edge direction and a pre-write hook, and its own capacity check runs after the entity write, orphaning the entity on fs/mem.
severity: significant
resolution: The converter now calls CreateLinked. CreateLinked gained Authored, RelateFrom/RelateFromFace for an edge to the new entity, and a BeforeWrite hook that runs in the transaction.
status: addressed
---
