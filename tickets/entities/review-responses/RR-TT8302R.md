---
id: RR-TT8302R
type: review-response
title: Phone test does not pin the breakpoint
finding: The test ran at 400px, so moving the media query to 420px would still pass.
severity: minor
resolution: 'The test now runs at 480px, the inclusive edge of max-width: 480px.'
status: addressed
---
