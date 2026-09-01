---
id: RR-5EPBE8
type: review-response
title: Unrelated quote-style churn on a CSS selector line
finding: input[type="text"] became input[type='text'] in FilterBar.vue, which is unrelated to this bug and makes git blame on that line misleading.
severity: nit
reason: Correct observation, but the change is mandated by the project's Prettier config — reverting it fails `prettier --check`, which is a CI gate. The line is adjacent to CSS this change genuinely removes (the dead select[multiple] rules), so the formatter reflowed it. Leaving as-is; changing the formatter's style rules is out of scope for a bugfix.
status: wont-fix
---
