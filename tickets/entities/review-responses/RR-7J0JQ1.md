---
id: RR-7J0JQ1
type: review-response
title: Dark kit tokens must go in both dark blocks
finding: dark.css has a .dark block and a prefers-color-scheme block (used by custom apps). Adding the mirrored kit tokens to only one leaves apps on a dark OS with light shades.
severity: significant
resolution: 'Plan changed: the generator writes the kit tokens into both dark blocks.'
status: addressed
---
