---
id: RR-OD7VF7
type: review-response
title: Missing picker and duplicate tests
finding: No tests for a failing widened fetch; a fallback-served duplicate source; no-default world; several target types; handleDuplicated routing.
severity: minor
resolution: Added the failing-fetch and stand-in duplicate tests and a default-flag widenWorlds test. The no-default and multi-type paths run through the same merge as the covered cases; handleDuplicated routing is covered by the faces-backlog e2e duplicate spec.
status: addressed
---
