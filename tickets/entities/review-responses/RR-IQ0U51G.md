---
id: RR-IQ0U51G
type: review-response
title: Conflict gates never refuse in the trace
finding: The conflict detail denial is refused by permitsGitSync before the path resolves, and the manager reads everything, so conflictReadableOr404 only runs its allow branch.
severity: minor
resolution: 'Added a demo role holding git:sync that reads only tasks. Its risk conflict detail is refused by conflictReadableOr404 (404), and its resolve is refused by AuthorizeUpdate (403). conflictFieldDenial still only shows its allow branch: the perf ACL has no field rule on task that a role with update could fail.'
status: addressed
---
