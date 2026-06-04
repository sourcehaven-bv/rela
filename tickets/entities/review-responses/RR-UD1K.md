---
id: RR-UD1K
type: review-response
title: 'mode: ''inline-edit'' reserved word leaks future scope into this contract'
finding: |
  Negative test says "widget receiving mode:'inline-edit' falls through to edit mode with no error." That's a soft API commitment to a value the type doesn't declare. Either declare the union as 'display' | 'edit' | 'inline-edit' now (and leave inline-edit widgets unimplemented), or restrict the type strictly to 'display' | 'edit' and force TKT-IHCY7 to widen it. The middle ground -- accept unknown strings silently -- invites typos that compile.
severity: minor
resolution: |
  Plan revised. WidgetProps.mode is strictly typed as 'display' | 'edit' -- the negative test for 'inline-edit' fall-through is gone (TypeScript catches it at compile time). TKT-IHCY7 will widen the union when it implements that mode, which is the right trigger for re-examining every widget consumer. No soft commitment.
status: addressed
---
