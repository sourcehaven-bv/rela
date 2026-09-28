---
id: RR-SZX2GP
type: review-response
title: Plural registry is never reset in the world test
finding: registerEntityPlurals in EntityDetail.world.test.ts mutates a module-level registry that is not reset.
severity: nit
reason: Other SPA tests register plurals the same way; entries are additive and idempotent, so no test depends on their absence.
status: wont-fix
---
