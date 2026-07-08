---
id: RR-7UJUAI
type: review-response
title: Prefer native title tooltip over custom click-expand (a11y + propagation risk)
finding: 'The row is the interactive element (tabindex, @keydown.enter, @keydown.space.prevent -> onIssueClick; AnalyzeView.vue:222-225). A custom inner click-expand with @click.stop contains the mouse click, but (a) it won''t be keyboard-reachable unless given role=button + tabindex, and (b) keyboard users pressing Enter/Space on the row still navigate, never expand. Simplest robust answer: use the native title attribute on the message cell for the missing-header list (hover tooltip, zero JS, no propagation/a11y risk). This satisfies the user''s ''mouseover'' ask. If click-expand is kept, it needs a real focusable button. Recommendation: ship the native title tooltip; treat click-expand as optional polish only if a focusable button is added.'
severity: significant
resolution: 'Resolved via user-directed split cell targets (supersedes the native-title-tooltip proposal): the entity-title cell navigates (onEntityClick), the message cell toggles a full-width colspan detail row (onMessageClick) and also owns the script-error dialog. Row-level click removed. Each target is an independently focusable role=button (tabindex, aria-expanded on message), so keyboard a11y works for both with no @click.stop propagation dance. Plan step 6.'
status: addressed
---

See finding property.
