---
id: RR-UIL8J
type: review-response
title: '''Previous value to revert to'' is undefined when keystrokes overlap a 422'
finding: |-
    Plan: 'On 422 response, the value reverts and an error toast is shown.' Scenario: user types 'abc' → debounce → PATCH → user types 'abcd' → 422 arrives for the 'abc' PATCH. Revert to what? The value before 'abc' clobbers 'abcd'. Revert to last-acknowledged-good loses both. Don't revert at all leaves UI showing 'abcd' while toast says save failed.

    Definition needed: 'previous' = value at the time the failed PATCH was *enqueued*. If a newer keystroke has arrived (newer debounce timer queued or newer PATCH in-flight for the same property), do NOT revert — newer input takes precedence. Only revert when the failed PATCH represents the latest user intent. Hold the toast regardless.
severity: significant
resolution: 'useAutoSave maintains pendingValue: Record<prop, {value, enqueuedAt}>. On 422, revert decision tree: if no newer keystroke (pendingValue absent or enqueuedAt <= failedAt), revert to lastSeenServer; if newer edit exists, do not revert (newer intent supersedes). Always show sticky toast. AC #5 covers both branches.'
status: addressed
---
