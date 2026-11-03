---
id: RR-V99XAQ
type: review-response
title: Hidden-field filter/sort leak not in corpus
finding: filter[] and sort on hidden fields disclose values (BUG-WJSYQT); the corpus has no case, so the baseline would miss the fix and any change.
severity: significant
resolution: Cases BUG-WJSYQT-list-filter-hidden, -list-filter-hidden-eq and -list-sort-hidden record today's leak; item 5a in the queue fixes it.
status: addressed
---
