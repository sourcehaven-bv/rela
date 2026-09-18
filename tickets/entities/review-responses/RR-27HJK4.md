---
id: RR-27HJK4
type: review-response
title: Guard gate was file-granular and blind to window.addEventListener
finding: |-
    Two blind spots in the positive assertion.

    1. The skip was `if (/isInputFocused/.test(text)) continue` — file-granular, not handler-granular. A file already importing the guard for one handler got a blanket pass for every other handler in it. That is precisely the shape of the bug being fixed: SearchView.vue is a 300+ line view with an existing multi-key switch, so a second unguarded shortcut added there would have been invisible. The scan would have missed its own recurrence.

    2. The gate required `document.addEventListener`. `window.addEventListener('keydown', ...)` is equivalent for a global shortcut, bubbles identically, and already appears in this codebase at router/index.ts:213. A new window-level shortcut was entirely invisible.
severity: significant
resolution: |-
    1. Replaced the file-level skip with per-handler analysis. `handlerBodies()` brace-matches each function/arrow body, and the guard call must appear in the SAME body as the bare-printable-key check. Verified against a synthetic second unguarded handler added to Sidebar.vue (which imports the guard for its first handler) — now caught; previously missed.

    2. Gate widened to `(?:document|window)\.addEventListener`. Verified with a probe component registering a window-level `z` shortcut — now caught.

    The reviewer suggested noting the file-granularity limitation in the docstring rather than building an AST walker. Brace-matching turned out to be ~20 lines and closes the hole properly, so I implemented it instead; the docstring records that it is crude and that ESLint AST selectors remain the rigorous option.
status: addressed
---

Finding 3 from the cranky-code-reviewer. Both blind spots verified present
before the fix and absent after.
