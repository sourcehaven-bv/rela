---
id: RR-2JIQLON
type: review-response
title: Copying an entity copies edge meta without the meta check
finding: CopyEdgeGate checks only create and remove, so copied edges carry meta the copier may not write.
severity: nit
reason: The copier cannot choose the values; the copy carries the stored edge as it is. Refusing it would block copies for every restricted role without protecting any value.
status: wont-fix
---
