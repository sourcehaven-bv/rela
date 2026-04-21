---
id: RR-3ON0U
type: review-response
title: Tests didn't exercise filepath.Rel-fails fallback in relativize
finding: The defensive fallback in relativize had no test coverage. Untested defensive branches rot.
severity: minor
resolution: 'Resolved by removing the fallback entirely (see #2). The code path no longer exists; no branch to test.'
status: addressed
---
