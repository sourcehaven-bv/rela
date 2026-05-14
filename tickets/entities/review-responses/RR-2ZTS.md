---
id: RR-2ZTS
type: review-response
title: 'Cranky #10: TestCreate_WritesTwice doesn''t pin creates count'
finding: Test name says 'writes twice' but only asserts updates == 1. A change from upsert-on-conflict to 'check-then-write' would silently flip creates from 2 to 1.
severity: minor
resolution: Added `if got := cs.creates.Load(); got != 2` assertion to pin both halves of the shape.
status: addressed
---
