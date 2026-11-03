---
id: RR-9JYOPF
type: review-response
title: Bare id can subscribe while schema loads
finding: EntityDetail.vue presence target treats an undefined typeDef as unfaced and may subscribe a bare id on a faced type; the 400 fails the whole subscription set
severity: minor
resolution: 'Fixed in SPA presence: review fixes'
status: addressed
---
