---
id: RR-RCEKR3
type: review-response
title: Lowercasing operator labels without a locale
finding: PilePanel, PageView, DuplicateModal and InlineCreateFormModal pass label.toLowerCase() into translated messages; toLowerCase ignores the locale.
severity: minor
resolution: Use toLocaleLowerCase(intlLocale()) at the four sites.
status: addressed
---
