---
id: RR-YV8D99
type: review-response
title: Read check restates roleGrantsRead
finding: 'roleReadFaces alone answers whether a role reads a face: all or the face is listed. The extra roleGrantsRead call is redundant.'
severity: minor
resolution: The read case uses roleReadFaces alone.
status: addressed
---
