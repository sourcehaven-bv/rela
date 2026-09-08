---
id: RR-QX7IGD
type: review-response
title: Non-SVG sanitizer input rendered as literal text
finding: RETURN_DOM_FRAGMENT on garbage yields a text node that was appended to the page.
severity: minor
resolution: A fragment whose first element is not <svg> yields an empty host; test asserts textContent is empty.
status: addressed
---
