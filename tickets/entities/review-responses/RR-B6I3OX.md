---
id: RR-B6I3OX
type: review-response
title: Duplicate view config fixtures in two test files
finding: KanbanConfig/CalendarConfig fixtures are near-identical in two tests.
severity: nit
reason: npm run dupes ignores test files; the fixtures are small and each test file reads on its own.
status: wont-fix
---
