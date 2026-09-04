---
id: RR-EPVQJB
type: review-response
title: 'PR-A: conformance suite gaps - no reopen case, unpinned aggregates, flaky event receive, no fs observer case'
finding: 'The suite missed exactly the places the three defects lived: no persistence/reopen case (storetest factories can''t reopen; fsstore had no state persistence test), PropertyValues/HighestID state-exclusion unpinned, EventsFirePerState used a non-blocking select+default that would flake on async backends (pg change-feed bridge) once un-gated in PR-B, and the observer skip was pinned only for memstore''s put path.'
severity: significant
resolution: 'Added: fsstore TestStatePersistence_FamilySurvivesReopen + TestStatePersistence_StateWriteFreshensCache; DefaultWorldAggregates conformance case with a count-ORDER-sensitive assertion (states leaking into counts flips closed/open ranking) + HighestID; EventsFirePerState now uses a bounded blocking receive with a 5s timeout; fsstore TestObservers_SkipNonDefaultStates + TestObservers_HeadlessRenameSkips cover put/update/rename incl. the headless path.'
status: addressed
---
