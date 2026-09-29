---
id: RR-XY4GYA
type: review-response
title: World resolution errors reported as entity not found
finding: Source and backend errors collapsed into 'entity not found' in the tools.
severity: minor
resolution: entityReadFailed distinguishes ErrNotFound (also the hidden answer) from other errors, which are logged and answered generically. TestEntityReadFailed.
status: addressed
---
