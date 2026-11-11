---
id: RR-DI9T66A
type: review-response
title: NewFile with a reader makes the store do slow I/O inside Tx
finding: A reader consumed inside Append runs inside the pg advisory-lock transaction or sqlite BEGIN IMMEDIATE.
severity: significant
resolution: NewFile carries Data []byte already processed and capped by the service; the store only checks caps.
status: addressed
---
