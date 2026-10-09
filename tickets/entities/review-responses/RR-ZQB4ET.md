---
id: RR-ZQB4ET
type: review-response
title: Mentions step re-runs every fixpoint pass
finding: executeView re-applies every traverse rule each fixpoint pass (views.go:155-164), so the MentionedBy scan would run at least twice per view.
severity: significant
resolution: The backlinks step runs once before the fixpoint loop; applyViewTraverse skips mentioned_by rules. TestQueryBudget_MentionsViewIsSizeIndependent asserts one MentionSources call.
status: addressed
---
