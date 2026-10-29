---
id: RR-EDUX0N
type: review-response
title: An unknown or ephemeral message type on a list is reported at [0] and hides the problems of the later entries
finding: branches.go:84-88. compile returns at the first entry whose transition() reports ok=false. That failure is a property of the message type, not of the entry, yet the problem is keyed on.<type>[0] instead of on.<type>, which is the key the mapping form uses. Problems in entries [1..n], such as a missing to or a bad role, are not reported in that load, so the operator fixes them over several rounds. Check the type once at key before the loop (the existing switch in transition() can move into a helper), then compile every entry.
severity: minor
resolution: transitionTypeProblem checks the message type once at on.<type> (the mapping form's key), and every entry still compiles, so entry problems show in the same load (new "unknown type reported at the key with entry problems" case).
status: addressed
---
