---
id: RR-FT7A57
type: review-response
title: A schema reload repopulated the type list behind a closed menu
finding: '`setAvailableTypes` called `refreshTypeItems()` unconditionally. On a closed menu `query` is '''', which is below MIN_SEARCH_LEN, so it took the `typeItems = availableTypes` branch and filled the list. `useSchemaMentionMenu` watches a `.map()` over `entityTypeList`, which allocates a fresh array on every evaluation, so Vue''s reference comparison fired the watcher on any schema store mutation (SSE, a settings change) and refilled the list behind a closed menu. Latent today because visibility is driven by the slash provider rather than by `state.open`, but it made `close()` a liar and the next person to gate rendering on `typeItems.length` would get a menu that opens by itself.'
severity: significant
resolution: '`refreshTypeItems` now early-returns with an empty list when `!state.open`, and `setAvailableTypes` returns early when the incoming list is element-wise equal to the current one (`sameNames`), so an unrelated store mutation no longer churns menu state. Also added the missing `if (disposed) return` to `setAvailableTypes`, which was the only controller mutator lacking the guard the others all had. Pinned by ''does not repopulate the type list behind a closed menu'', ''ignores setAvailableTypes after dispose'' and ''is idempotent for an unchanged type list''.'
status: addressed
---
