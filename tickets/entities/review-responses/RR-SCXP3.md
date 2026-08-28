---
id: RR-SCXP3
type: review-response
title: waitForRelationVersions silently guessed plural→singular type
finding: 'waitForRelationVersions hardcoded a {features,bugs,tasks} plural→singular map with a `?? fromPlural` fallback. A new entity type would silently build a wrong _relation_history path, 404 every poll, and fail with a generic timeout pointing the debugger at the sweep, not the typo.'
severity: significant
resolution: 'The helper now takes the singular fromType explicitly (the _relation_history path needs it), removing the guess/fallback entirely. Callers pass "feature". Also added doc notes: pid/counter schema-name uniqueness rests on Playwright''s separate-process workers; pgDsnForSchema overrides any pre-existing options param.'
status: addressed
---
