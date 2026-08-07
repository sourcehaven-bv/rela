---
id: RR-7JFJUB
type: review-response
title: Spacing and shadow tokens ship with zero call sites, including an unexercised dark-mode branch
finding: |-
    grep for var(--space- and var(--shadow- across frontend/src returns 0 hits. Nine spacing tokens and three shadow tokens (six declarations counting the :root.dark override) are defined and used nowhere, while 60 box-shadow literals remain in the tree.

    Two of those literals sit directly adjacent to lines this commit edited:
    - EntityList.vue:1138 `box-shadow: 0 1px 3px rgba(0,0,0,0.1)` -- the line immediately after a border-radius this commit tokenized. That is --shadow-sm.
    - EntityDetail.vue:1339 `box-shadow: 0 4px 12px rgb(0 0 0 / 12%)` on .overflow-menu, an overlay -- exactly what --shadow-lg is documented for.

    The specific hazard is that the :root.dark shadow override (scales.css:77-81) encodes a rendering claim -- that an 8%-alpha black shadow is invisible on a dark surface -- which no pixel in the app currently exercises. PR 2 will adopt it assuming it was validated here. It was not.

    Resolve either way, but deliberately: migrate the two adjacent shadows so the dark branch becomes real (2-line change in files already touched), or drop the spacing/shadow blocks and land them with their first consumer. Defining a dozen tokens with no call sites is how a design vocabulary nobody speaks accretes.

    Related smaller instances: --radius-pill is unused while two literal 999px remain (DynamicForm.vue:1641, ScriptErrorPanel.vue:123, both outside the migration scope).
severity: minor
resolution: |-
    Fixed in 27bc6ded by taking the reviewer's first option: adopt the tokens rather than defer them. --shadow-sm and --shadow-lg are now used at the two adjacent literals (EntityList .list-content, EntityDetail .overflow-menu), so the :root.dark override is exercised rather than dead.

    Importantly, the token VALUES were tuned to match the shadows already in the tree (--shadow-sm = 0 1px 3px 10%, --shadow-lg = 0 4px 12px 12%) rather than the invented values, keeping adoption value-preserving — verified in-browser: .list-content computes to rgba(0,0,0,0.1) 0px 1px 3px 0px, byte-identical to the pre-migration literal.

    The spacing scale also had zero call sites, so 59 single-value `gap:` declarations across the same nine components were migrated (shorthand like `gap: 16px 32px` deliberately left alone). All 72 gap/shadow declarations verified value-preserving.

    --radius-pill remains unused: its only two call sites (DynamicForm.vue, ScriptErrorPanel.vue) are outside this PR's migration scope, and pulling them in would widen the diff for no benefit. It is a one-line definition that PR 2 will consume.
status: addressed
---
