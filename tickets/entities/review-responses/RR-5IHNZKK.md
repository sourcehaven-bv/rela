---
id: RR-5IHNZKK
type: review-response
title: "An applied proposal does not refresh the panels"
finding: "Applying an llm proposal reloaded the view only, so the Changes list and the other panels stayed stale after it."
severity: significant
resolution: "Addressed: One helper, reloadAfterWrite, reloads the view and bumps the panel revision for both an action and an applied proposal. Pinned in EntityDetail.actions.test.ts."
status: addressed
---
