---
id: RR-7PZJM2
type: review-response
title: Panic test ignores the application log
finding: The panic path was not checked for keeping records out of the app log.
severity: nit
resolution: Panic test asserts the application log stays empty.
status: addressed
---
