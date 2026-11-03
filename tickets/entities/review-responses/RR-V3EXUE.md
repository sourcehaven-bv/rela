---
id: RR-V3EXUE
type: review-response
title: Sidebar logs WARN for an expected property_not_visible refusal
finding: 'nav_items.go: errPropertyNotVisible logs WARN on every sidebar render.'
severity: minor
resolution: 'nav_items.go and nav_status.go log a skipped entry at sidebarSkipLevel(err): DEBUG for errPropertyNotVisible, which the operator now gets as a load-time warning, and WARN for any other cause.'
status: addressed
---
