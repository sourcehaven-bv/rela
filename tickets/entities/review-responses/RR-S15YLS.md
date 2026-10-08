---
id: RR-S15YLS
type: review-response
title: Windows reserved-name rule lives in principal
finding: Storage concern in identity package; agreement test samples only.
severity: nit
reason: 'The reviewer judged it acceptable as is: the reserved-name rule must hold for principal names and token names alike, and a test pins agreement between the two packages. Moving it to a shared leaf package adds a dependency for one small function.'
status: wont-fix
---
