---
id: RR-P1YJ4W
type: review-response
title: time.Local in IsDue tests is machine-timezone-dependent
finding: All timestamps in the new test use time.Local, matching the surrounding tests. Weekday-of-date is zone-invariant so the assertions hold globally, but a future UTC-derived or DST-adjacent row would diverge between CI and laptops.
severity: nit
reason: Consistent with the file's existing convention for every IsDue test; churning only the new test would make it the odd one out. Noted for a future file-wide hermetic-timezone cleanup if a DST-sensitive case is ever added.
status: wont-fix
---
