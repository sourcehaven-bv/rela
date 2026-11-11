---
id: RR-46BEVRX
type: review-response
title: Tab title stale in hidden tabs
finding: Summary refetch is runWhenVisible, so the (N) title would not change in a background tab.
severity: significant
resolution: Elected tab reads the default inbox page and writes it with storeInboxSummary, updating summary, sidebar badge and title in one request; driven by the summary live key. Non-elected hidden tabs stay stale (AC1 reworded).
status: addressed
---
