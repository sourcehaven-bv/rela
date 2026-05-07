---
id: RR-F06WV
type: review-response
title: JSON pointer error messages don't escape user input
finding: |-
    api_v1.go:588, 647, 654 use %s for user-supplied relType in error pointers. A relType containing slash, percent, or newline produces broken pointers. Not a security issue (response goes through JSON encoding), but downstream consumers parsing the pointer get garbage.

    Fix: use %q for the relType segment, or escape per RFC 6901.
severity: nit
resolution: 'All JSON pointer error messages now use %q for the relType segment. Example: ''/relations/%q/data/%d/type: type field required'' — a relType containing slashes/percent/newline produces a properly-escaped Go-quoted string in the pointer.'
status: addressed
---
