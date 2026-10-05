---
id: RR-NLDWBN
type: review-response
title: 'PR 8: schema usage counter counts face rows'
finding: store_adapter counts rows across every face, so a family at three faces counts three.
severity: minor
resolution: 'Documented the unit on NewStoreCounter: the usage analysis only asks none/few/many, and a single-world count would report a faced-only type as unused.'
status: addressed
---
