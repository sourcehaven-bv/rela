---
id: RR-RG2HCV
type: review-response
title: writeMarkup drops attribute namespaces and void-element content
finding: xml:lang becomes lang (duplicate); a void element with content loses it.
severity: minor
resolution: 'xml: attributes keep the prefix, other namespaced attributes are dropped; void elements only self-close when empty. Tested.'
status: addressed
---
