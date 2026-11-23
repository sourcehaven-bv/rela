---
id: RR-K1EAB4H
type: review-response
title: Fold refusal tested only with a principal-blind fake resolver
finding: The hidden-value test used fakeResolver, which hides points from everyone, so it never showed that the real redactor reads the Request the scheduler bound.
severity: significant
resolution: 'TestCardFigure_HiddenValueFollowsTheBoundPrincipal uses the real affordances resolver over a policy with a visible: block: alice (points hidden) is refused, bob gets the figure.'
status: addressed
---
