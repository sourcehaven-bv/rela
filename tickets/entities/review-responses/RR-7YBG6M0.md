---
id: RR-7YBG6M0
type: review-response
title: 'Nits: title cut, preview attributes, IME Enter, handle focus, wall title fallback'
finding: Title cut could split a surrogate pair; preview shell copied id/role/aria-label; Enter blurred during IME composition and Escape did not revert; the handle was focusable but keyboard-inert; a plain wall without title had an undefined section title.
severity: nit
resolution: Cut by code points (test added); preview copies only class and data-v-*; Enter skips isComposing and Escape restores the name; handle has tabindex=-1 with a comment; plain wall falls back to the default board label.
status: addressed
---
