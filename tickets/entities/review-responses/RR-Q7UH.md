---
id: RR-Q7UH
type: review-response
title: EntityPickerModal is a near-verbatim copy of CommandPaletteModal — drift risk in 6 months
finding: |-
    Side-by-side diff: ~280 lines of script + template + styles are duplicated between `CommandPaletteModal.vue` and `EntityPickerModal.vue`. The differences are: (1) `selectEntity` emits `select(id)` instead of `router.push`; (2) `aria-label` text; (3) CSS class prefix `cmdk-` → `entity-picker-`; (4) z-index 1000 → 10000. Everything else (debounce, MIN_QUERY_LEN, MAX_RESULTS, abort logic, focus restore, keyboard nav, modal stack registration, listbox id randomization, accessibility attrs) is identical.

    The code comment says 'kept as a sibling rather than a generalization … to avoid regression risk in the Cmd+K flow — a future ticket can DRY them when a third consumer appears.' That is the standard rationale for the 'Rule of Three', but it ignores the cost: any future change to either component (a11y improvement, debounce tuning, new keyboard shortcut, stale-results UX, search cancellation policy) now has to be applied in two places. CommandPaletteModal already accumulated subtle behavior over time (sync flush watcher, scrollHighlightedIntoView, cancellation-on-close); the next iteration on either component will quietly diverge and only one user-facing flow will get the improvement.

    Concrete drift surfaces already visible:
      - CommandPaletteModal has a `// 8-char random suffix` comment; EntityPickerModal has `// Random suffix so multiple Teleport-mounted listboxes don't collide` (different wording, same code).
      - CommandPaletteModal has detailed multi-paragraph comments explaining each block; EntityPickerModal has shorter comments.
      - `flush: 'sync'` rationale comment is fully in CommandPaletteModal, condensed in EntityPickerModal.
      - CSS animations are renamed (`cmdk-spin` vs `entity-picker-spin`) and now BOTH keyframes run independently if both modals were ever open.

    Recommended refactor (next ticket, NOT a blocker for I5NO): extract `<EntitySearchPalette :open :on-pick="fn">` with a slot/prop for the action. CommandPaletteModal becomes a thin wrapper that wires `onPick` to `router.push`; EntityPickerModal becomes a thin wrapper that wires it to `emit('select')`. z-index difference becomes a prop. Total component size ~30 lines each, shared core ~200.

    Flagging as 'significant' rather than 'critical' because it's not a runtime bug — but the next bug fix to one of these will reveal the duplication tax.
severity: significant
reason: 'Deferred to a follow-up ticket. The reviewer''s recommendation -- extract a shared EntitySearchPalette core with action-as-prop -- is sound but out of scope for I5NO: it would mean refactoring the already-merged CommandPaletteModal alongside introducing the new one, doubling the blast radius. Mitigation now: pair the two components with explicit cross-comments in EntityPickerModal pointing at CommandPaletteModal so a future refactor can find both sites at once. The drift cost the reviewer cites is real but slow-moving (no behavior changes pending on either palette). A follow-up ''refactor: extract shared entity-search palette'' ticket is the right venue once a third consumer appears or the next a11y change is queued.'
status: deferred
---
