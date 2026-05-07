---
id: RR-30SPD
type: review-response
title: Plan only handles 422 and 5xx; ignores 404, 412, 400, 401, 403
finding: "Plan says 422 → revert + toast; 5xx → keep value, toast, no revert. But: 404 (entity deleted in another tab) means the form is editing a ghost — reverting one field is meaningless. 412 (If-Match mismatch) — won't fire today since frontend doesn't send ETags, but the family of non-422 4xx (400, 401, 403) all need a defined story. \n\nRule: 422 reverts (definitive validation rejection). All other 4xx and 5xx keep the user's value with a sticky toast and status='error' until next successful save; do not auto-retry."
severity: significant
resolution: 'Scope and AC #6 now spell out: 422 reverts (if latest intent); all other 4xx (404, 412, 400, 401, 403) keep value with sticky toast and status=''error'' until next successful save; 5xx + network failures behave the same. No auto-retry. Vitest test parameterized over status codes.'
status: addressed
---
