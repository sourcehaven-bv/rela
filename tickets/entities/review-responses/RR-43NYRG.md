---
id: RR-43NYRG
type: review-response
title: Duplicate of a stand-in face creates the copy on that face
finding: copyFace came from _self; a face served as a fallback or later chain entry sent face=<stand-in>; so an editorial duplicate of a published stand-in was created published and skipped the editorial step.
severity: significant
resolution: copyFace is empty when _world.via is fallback-default or chain_position > 0; the world then decides the face as before. Pinned by two DuplicateModal cases.
status: addressed
---
