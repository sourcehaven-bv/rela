---
id: RR-7YTW0
type: review-response
title: 'Significant: refuseIfTracked swallowed context-deadline-exceeded as ''not tracked'''
finding: 'cranky-code-reviewer #5: `if runErr := cmd.Run(); runErr == nil` branched only on success; any error (timeout, I/O, exec failure) was treated as ''not tracked'', i.e. safe. On a hung git that defeats RR-LDRW3.'
severity: significant
resolution: 'Inspect the error: only a clean *exec.ExitError (non-zero exit) is the ''definitely not tracked'' signal; any other failure mode returns a wrapped error surfacing the ambiguity. Timeout now propagates upward rather than silently greenlighting.'
status: addressed
---
