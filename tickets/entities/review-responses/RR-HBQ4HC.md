---
id: RR-HBQ4HC
type: review-response
title: '[security] 404 cases did not assert the body matches a real not-found'
finding: '[security] Only status codes were compared, so a distinct denial message would not be caught as an existence oracle.'
severity: minor
resolution: Each route first requests TKT-999@draft; every 404 case asserts its body equals that one (instance stripped).
status: addressed
---
