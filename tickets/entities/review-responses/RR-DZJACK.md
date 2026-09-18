---
id: RR-DZJACK
type: review-response
title: Generalizing the resolver would ship an ungated link-existing affordance onto _views, which is out of scope
finding: resolveSectionButtonsWithTraverse also builds SectionLinkInfo unconditionally whenever len(candidateTypes) > 0 (sections.go:517-525) — with no form check and no permission check. "Linking existing entities" is explicitly out of scope for this ticket, so generalizing the function wholesale to the _views path would silently ship an ungated link affordance onto a new surface.
severity: significant
resolution: Plan now requires the function be split so _views receives only the add path, or that both paths be gated. Recorded explicitly as "do not let it ride along" so the implementation cannot take the lazy route of moving the whole function.
status: addressed
---

## Finding

The same function the plan proposes to generalize also builds `SectionLinkInfo`,
unconditionally, whenever `len(candidateTypes) > 0` (`sections.go:517-525`) —
with no form check and no permission check.

"Linking existing entities from the detail page" is explicitly out of scope for
this ticket. Generalizing the function wholesale would therefore ship an
**ungated** link affordance onto `_views` as a side effect of adding create
buttons.

## Resolution

Plan now requires either splitting the function so `_views` receives only the
add path, or gating both. Recorded as "do not let it ride along", so the
implementation cannot take the lazy route of moving the whole function and
inheriting its second affordance unnoticed.
