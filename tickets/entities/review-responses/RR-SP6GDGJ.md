---
id: RR-SP6GDGJ
type: review-response
title: Manual scroll ignores scroll-margin and scroll-padding
finding: The first version computed scrollTop by hand; scrollIntoView honours scroll margins and the manual offset did not.
severity: significant
resolution: The manual scroll is gone; focusRunCard keeps scrollIntoView.
status: addressed
---
