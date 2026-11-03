---
id: RR-EDRNWVD
type: review-response
title: Moved configDraft test uses alias imports for siblings
finding: configure/__tests__/configDraft.test.ts imported its siblings through @/configure/... while every other test in that folder uses relative paths.
severity: nit
resolution: Changed to ../configDraft, ./fixture and ../tree.
status: addressed
---
