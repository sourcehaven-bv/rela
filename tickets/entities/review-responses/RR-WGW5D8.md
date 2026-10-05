---
id: RR-WGW5D8
type: review-response
title: pg DeleteEntityState locks the face='' row which a faced family lacks
finding: pgstore DeleteEntityState serializes with SELECT ... WHERE id=$1 AND face='' FOR UPDATE. For a faced family that locks nothing so a concurrent face create can commit between the faces-left count and the attachment sweep and lose the family's attachments. The section 3 face='' audit misses it.
severity: significant
resolution: 'Amendment A2: PR 3 switches the lock to lockFamily (the advisory lock DeleteEntity uses) with a pg barrier race test under -race.'
status: addressed
---

## Finding

pgstore DeleteEntityState serializes with SELECT ... WHERE id=$1 AND face='' FOR
UPDATE. For a faced family that locks nothing so a concurrent face create can
commit between the faces-left count and the attachment sweep and lose the
family's attachments. The section 3 face='' audit misses it.

Design: `.ignored/stage2-design.md` section 11.
