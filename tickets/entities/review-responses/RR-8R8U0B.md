---
id: RR-8R8U0B
type: review-response
title: '!important precedence inverts under @layer, making the ticket''s headline promise false'
finding: 'TKT-3DBK6I states ''unlayered operator CSS outranks ALL layered CSS regardless of source order''.
  VERIFIED FALSE for !important. In layered CSS, !important precedence is REVERSED: an important declaration
  inside a layer beats an important declaration outside it. Measured in a browser: rela''s layered ''.t
  { color: red !important }'' vs an operator''s later, unlayered ''.t { color: green !important }'' resolves
  to RED — operatorImportantWins: false. rela has exactly 17 !important declarations (verified count)
  at App.vue:336, RelationCards.vue:1340-1343, MarkdownEditor.vue:286-377. So the one tool an operator
  reaches for when normal CSS does not work is precisely the tool that stops working. This is a documentation-vs-reality
  contradiction that would ship inside AC6.'
severity: critical
status: addressed
resolution: 'Documented in docs/customisation.md under "The one exception: !important", stating the inversion
  plainly and that there is no CSS-only workaround. Also recorded in frontend/CLAUDE.md so nobody "fixes"
  it by unlayering. Verified in a browser: rela layered !important beats operator unlayered !important.'
---

## Failure scenario

Operator cannot override the portaled `.ss-content` dropdown border
(`RelationCards.vue:1340`). They try `.ss-content { border: none }` — loses to
rela's `!important`. They escalate to `.ss-content { border: none !important }`
— **still loses**, because rela's important rule is layered and theirs is not.
They conclude custom CSS is broken. The documented rule says they should win.

## Verification

Direct browser measurement, not inference:

| rule | computed |
|---|---|
| rela `@layer rela { .t { color: red !important } }` | **wins** |
| operator (later, unlayered) `.t { color: green !important }` | loses |

`relaLayeredImportantWins: true`.

## Recommended resolution

AC6 docs **must** state the `!important` inversion explicitly — it is a genuine,
permanent property of the chosen design, not a bug. Additionally audit the 17
`!important` uses: several (the `MarkdownEditor.vue` z-index stack) exist only
to beat EasyMDE and may be removable once EasyMDE is itself layered, which would
shrink the trap.

Relates to [[rr-vendor-css-in-layer]].
