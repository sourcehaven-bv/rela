---
id: RR-9T6J5B
type: review-response
title: A14 false positives
finding: A14 treats wildcard update/create as covering every face, treats a type@face grant as opening the block on all faces, and lets create neutralise a fields block on update.
severity: minor
resolution: 'A14 now runs through acl.EveryoneOpenedFieldBlocks: per declared face (MetamodelReader.Faces) it asks acl.GrantsVerbOnFace with the dimension verb: read for visible; update and create separately for fields and options. So `*` update no longer covers named faces; type@face grants report only that face; a create grant reports a create finding only. Tests cover each case and the face list in the detail.'
status: addressed
---
