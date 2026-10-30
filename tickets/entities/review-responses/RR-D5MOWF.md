---
id: RR-D5MOWF
type: review-response
title: 'No-effect warning fires next to the error of a broken authored in:'
finding: 'internal/realtimecfg/message_in.go:68-71 and 77-87: when a keyed type''s authored in: has a problem, one returns ok=false. derived stays 0, so with default_in: keyed the load also warns "every message type a transition is keyed by declares in:; drop the in: that only repeat on: keys". The operator then sees a load error plus a warning that tells them to remove the in: they are fixing. Fix: track whether any authored in: under the key failed, and skip the warning when one did. Or count a type as "authored" for the warning only when one returned ok. Add a case to TestDefaultIn_Checks: an in: that misses a keyed state gives exactly one problem and no default_in warning.'
severity: minor
reason: 'The premise does not hold: Parse returns only Error{Problems} when the load fails and drops the validator warnings, so an operator never sees the warning next to the error. Tracking failed authored in: would add state for an output nobody can read.'
status: wont-fix
---
