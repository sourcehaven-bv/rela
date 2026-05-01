---
id: RR-3J2MJ
type: review-response
title: Duplicate saveAndWaitForNavigation across page objects
finding: FormPage.saveAndWaitForNavigation is a verbatim copy of RelationCardsPage.saveAndWaitForNavigation (e2e/pages/form.page.ts:301-306 vs e2e/pages/relation-cards.page.ts:118-123). Both will drift the next time the navigation contract changes. Move to BasePage and have both POs delegate.
severity: significant
resolution: Extracted submitFormAndWaitForNavigation(submitButton) to BasePage. Both FormPage.saveAndWaitForNavigation and RelationCardsPage.saveAndWaitForNavigation now delegate to it. Single source of truth for the navigation predicate.
status: addressed
---
