---
id: RR-1SQI5QX
type: review-response
title: 'A duplicate transition skips its when: compile'
finding: The continue after the duplicate problem skipped compiling that entry's when:, so one boot reported fewer problems.
severity: nit
resolution: 'Addressed: when: compiles before the duplicate check, so every problem shows in one boot.'
status: addressed
---
