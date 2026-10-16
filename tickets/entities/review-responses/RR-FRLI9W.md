---
id: RR-FRLI9W
type: review-response
title: External-create restore may attach old channels to a new entity
finding: The reconciler restores ledgered channels on any external EventEntityCreated for the id, without proving it is the same entity; a different entity created out of band (or through another process's manager) with that id inherits them.
severity: minor
status: open
---
