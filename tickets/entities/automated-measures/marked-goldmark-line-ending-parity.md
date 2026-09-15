---
id: marked-goldmark-line-ending-parity
type: automated-measure
title: 'Test: marked agrees with goldmark on what terminates a line'
description: 'Control for GitHub #1594. flattenToLine replaces only \n and \r in an interpolated webhook value, leaving \v, \f, U+0085, U+2028 and U+2029 intact (rune set established by RR-2HD5RQ on TKT-02V29V). That is safe only because of what the PARSER treats as a line ending, so the guarantee belongs to the renderers rather than the function - and it had been verified against goldmark on the write path only. This suite checks the other renderer: for each survivor it asserts marked forges no heading from the webhook delivery shape and never drops the value text. A divergence on any one character would reopen the forged sibling-heading vector TKT-02V29V closed. Non-vacuous by construction: \n and \r positive controls assert a heading IS forged, so the negative cases remain evidence.'
kind: test
location: frontend/src/utils/markdownLineEndings.test.ts; internal/dataentry/webhook_routes.go (flattenToLine godoc)
status: active
---
