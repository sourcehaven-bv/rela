---
id: RR-RQLNJ6
type: review-response
title: Global reduced-motion rule cannot reach any scoped spinner
finding: |-
    The headline accessibility win was mostly not real. `styles/pending.css` is unscoped, but every spinner it named except App.vue's `.spinner` is declared inside a `<style scoped>` block, which Vue compiles to `.spinner-sm[data-v-abc123]` — specificity (0,2,0) against the stylesheet's (0,1,0). The component rule wins and the animation keeps running.

    Worse, my own response to discovering this made it worse rather than better: I had narrowed the e2e test to the single case that passes while ADDING `.search-spinner` / `.cmdk-spinner` / `.entity-picker-spinner` to a selector list that still could not win, and left a comment asserting the list 'covers every rotating busy affordance in the app'. A selector list that reads as complete and is not is worse than an absent one — the next reader believes the preference is handled and does not check. The claim that reduced-motion coverage went from 1-of-11 sites to universal was off by roughly ten sites.
severity: significant
resolution: |-
    Suppression now lives beside each declaration: `.search-spinner` (RelationCards), `.cmdk-spinner` (CommandPaletteModal), `.entity-picker-spinner` (EntityPickerModal) and `.spinner-sm` (DocumentView, DocumentsPanel) each carry their own `prefers-reduced-motion` rule inside their scoped block. `pending.css` keeps only `.spinner`, which is the one class it can actually reach.

    One of the five initially landed in RelationCards' UNSCOPED second `<style>` block, where it would have had the same specificity problem; caught by checking which block each rule landed in, and moved.

    The stylesheet comment now states the scope limit and WHY (the [data-v-*] specificity rule), so a future contributor adding a class to that list learns immediately that it will silently do nothing. `frontend/CLAUDE.md` gained the same rule, plus a note that an e2e probe element built with `createElement` only picks up unscoped CSS — which is what made the gap invisible in the first place. The e2e test now honestly asserts the global spinner only.
status: addressed
---
