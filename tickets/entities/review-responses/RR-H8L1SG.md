---
id: RR-H8L1SG
type: review-response
title: Integer preset outside the JS safe range breaks the load-time click analysis
finding: 'internal/realtimecfg/buttons.go:212 decodes a !!int preset to any int64, and NormalizeProps (internal/realtimecfg/props.go:221) keeps it exactly. The load evaluates every earlier guard against that exact value. The SPA reads the served props with JSON.parse, so a value over 2^53 rounds, and presetProps posts the rounded number back. internal/dataentry/channels_handler.go:576 decodes with UseNumber, so the server accepts the rounded value. The decider then evaluates the guards on a value the load never checked. Example: entry 0 has when: message.n <= 9007199254740992 and outputs, entry 1 has a button with n: 9007199254740993. The load finds entry 0 does not hold, so it does not hide the button. A click posts 9007199254740992, and entry 0 fires its outputs. This needs an operator-authored boundary preset, so it is minor. Fix: in buttonChecker.preset (internal/realtimecfg/buttons.go, after NormalizeProps at :155), refuse an integer preset with abs(n) > 1<<53 with a load problem saying the browser cannot carry the value exactly. A matching test belongs in buttons_test.go.'
severity: minor
resolution: buttonChecker.preset now refuses an integer preset whose absolute value is over 2^53 (the same bound normalizeInt applies to a JSON number), with a message that a browser cannot carry the value exactly. Tests cover 2^53+1, -(2^53+1) and that 2^53 itself loads.
status: addressed
---
