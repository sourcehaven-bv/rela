---
id: RR-47YK1
type: review-response
title: RelationCardsPage leaks Locator through public API
finding: widgetByLabel returns Locator; specs then chain Playwright-specific helpers on it. Defeats POP. Wrap as RelationWidget class with its own methods.
severity: significant
reason: Refactoring RelationCardsPage to hide Locator is a substantial redesign. Current API works and is documented; the leak is internal-enough that specs don't call Playwright selectors directly on the Locator — they pass it back to page-object methods. Defer to a focused tech-debt ticket.
status: deferred
---
