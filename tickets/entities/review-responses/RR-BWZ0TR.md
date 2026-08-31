---
id: RR-BWZ0TR
type: review-response
title: click.stop on the shared cell wrapper swallowed clicks on every non-first column
finding: |-
    The dedup refactor moved the anchor's `@click.stop` onto the wrapper shared by
    ALL columns. Vue resolves `.stop` at COMPILE time, so the ternary only chose the
    handler body while propagation was halted unconditionally. Non-first columns
    called nothing AND blocked the row's own handler. Verified before fixing:
    clicking the second column's wrapper produced 0 router pushes, against 1 on the
    pre-change markup. Masked by `display: contents` on `.row-cell` — the wrapper
    has no box, so clicking the cell PADDING still reaches the `<td>` and navigates;
    only the rendered glyphs go dead, which is exactly where users aim.
severity: critical
resolution: |-
    Fixed by moving every wrapper attribute into `rowCellWrapper()`, which returns
    the click handler (calling `stopPropagation()` itself) for the anchor and
    `undefined` for the plain columns — making it a runtime rather than a
    compile-time decision.

    Pinned by `still navigates when a NON-first cell is clicked`, mutation-verified:
    restoring the `@click.stop` form reddens that test and nothing else.

    Post-mortem: my own dedup refactor introduced this. The first version had the
    anchor and the plain span as separate `v-if`/`v-else` branches and was correct.
    Collapsing them to remove duplicated markup moved a modifier off one element
    onto all of them. No test caught it because every test clicked either the anchor
    or the row itself — never a non-first cell. That gap is now closed.
status: addressed
---

## Resolution

Fixed by moving every wrapper attribute into `rowCellWrapper()`, which returns
the click handler (calling `stopPropagation()` itself) for the anchor and
`undefined` for the plain columns — making it a runtime rather than a
compile-time decision.

Pinned by `still navigates when a NON-first cell is clicked`, mutation-verified:
restoring the `@click.stop` form reddens that test and nothing else.

Post-mortem: my own dedup refactor introduced this. The first version had the
anchor and the plain span as separate `v-if`/`v-else` branches and was correct.
Collapsing them to remove duplicated markup moved a modifier off one element
onto all of them. No test caught it because every test clicked either the anchor
or the row itself — never a non-first cell. That gap is now closed.
