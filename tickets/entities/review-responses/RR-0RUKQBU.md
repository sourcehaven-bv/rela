---
id: RR-0RUKQBU
type: review-response
title: Inert routes break EntityDetail load
finding: The plan answers 501 for every route outside scope. EntityDetail also loads commands, scope navigation, comments, presence, live subscriptions, transforms and actions through axios; loadCommands failing sets the page error state, so the test cannot reach the editing code.
severity: significant
resolution: The fake gets an explicit inert-route table with typed empty responses. 501 stays the default only for routes in neither the implemented nor the inert table.
status: addressed
---
