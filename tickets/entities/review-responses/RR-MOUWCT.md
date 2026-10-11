---
id: RR-MOUWCT
type: review-response
title: pg TagCurrent races synchronous delete/rename capture
finding: WriteVersion takes no lock and the candidate read runs READ COMMITTED; a delete or rename between scan and capture yields a capture row after the delete row and an inherited tag.
severity: significant
resolution: FOR SHARE OF e on the pg candidate query; storetest ConcurrentDeleteAndTag.
status: addressed
---
