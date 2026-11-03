---
id: RR-7ZUH31
type: review-response
title: Ratchet does not see non-listed props or binding literals
finding: The bare-string rule only checks text and the listed attributes (title, aria-*, placeholder, label, alt). Props such as empty-text, confirm-label or a literal inside a binding pass lint, so shrinking the allowlist does not prove a screen is translated.
severity: significant
resolution: Each area pass greps its files for quoted English in props, bindings and TS messages (toast, confirm, error) and converts them; code review checks the same.
status: addressed
---
