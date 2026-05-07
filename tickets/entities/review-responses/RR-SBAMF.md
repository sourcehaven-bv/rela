---
id: RR-SBAMF
type: review-response
title: newRels is dead code
finding: |-
    api_v1.go:553 declares `var newRels []*model.Relation`, line 738 appends to it inside the WithTx callback. Never read after the callback returns. Compiler doesn't warn on append.

    Fix: delete the variable and the append.
severity: nit
resolution: newRels variable removed. responseTypeName/responseEntityID aliases also removed; the broadcast call uses typeName/entityID directly.
status: addressed
---
