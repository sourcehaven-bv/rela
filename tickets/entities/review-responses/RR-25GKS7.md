---
id: RR-25GKS7
type: review-response
title: Removed out-of-page link cannot be re-added from the dropdown
finding: 'filteredCandidates offers candidates only, so an out-of-page link never appears in the dropdown. Pre-existing, but the fix widens its consequence: a link that was previously invisible is now visible and removable, turning it into a one-way door — the user can remove it and then cannot restore it without leaving the form.'
severity: significant
reason: 'The real fix is making the dropdown search hit the search endpoint rather than the cached candidate page, which is a separate change with its own scope and risk. Folding it into a bug fix about type resolution would mix two concerns. Accepted as-is with an acknowledging comment at filteredCandidates naming the asymmetry, so the next person reading it does not mistake it for an oversight. Note the fix is still a strict improvement: before it, the link was invisible AND unsaveable.'
status: wont-fix
---
