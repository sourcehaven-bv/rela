---
id: RR-DOWNKVM
type: review-response
title: Role union written twice
finding: '''previous'' | ''current'' | ''next'' appears in RlChannelSession.vue and useMessages.ts.'
severity: nit
reason: Two short literal unions in a library that keeps its message types self-contained; TypeScript checks the call.
status: wont-fix
---
