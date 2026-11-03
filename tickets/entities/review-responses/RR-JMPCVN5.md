---
id: RR-JMPCVN5
type: review-response
title: Section drag may block text selection in the rename input in Firefox
finding: draggable() sets draggable=true on the whole section including the rename input.
severity: minor
reason: Firefox is not in the e2e matrix and the feature is mouse-first for Chromium-based workshop use; tracked as a follow-up check before wider rollout.
status: deferred
---
