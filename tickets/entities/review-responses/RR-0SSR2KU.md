---
id: RR-0SSR2KU
type: review-response
title: '[security] Face filtering only in storage'
finding: '[security] A record of the id at another face could be listed when stored with an empty face.'
severity: minor
resolution: 'showable drops entity records at another face and relations leaving the id from another face (atFace). Test: TestEntityChanges_OtherFaceLeftOut.'
status: addressed
---

[security] A record of the id at another face could be listed when stored with
an empty face.
