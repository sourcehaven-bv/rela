---
id: RR-ERCH9
type: review-response
title: e2e hidden-branch test didn't exercise the leak path (false confidence)
finding: The 'drops hidden-branch values' e2e left the toggle unchecked the whole flow, so the conditional field never entered formData/userTouched. It proved the trivial 'a never-touched field isn't invented', not reveal->fill->hide->submit — which is exactly why the critical bug shipped.
severity: significant
resolution: 'Added ''a revealed-then-hidden field is NOT persisted on create'' e2e: fills assignee after revealing the step, hides it, submits, asserts assignee is absent. Now fails without the create-path fix and passes with it.'
status: addressed
---
