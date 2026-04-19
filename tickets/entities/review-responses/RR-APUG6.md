---
id: RR-APUG6
type: review-response
title: Step 6 MkdirAll consolidation is a separate refactor
finding: Step 6 widens StoreFS.WriteFile contract to 'auto-mkdir' — behavioral change in bottom of stack, not in encryption boundary. Bleeds into SafeFS (already mkdirs at safefs.go:32) and OsFS (doesn't). Cramming contract change into a refactor whose AC is 'byte-for-byte identical cleartext output' risks regressions far from encryption. Plan says 'Separate PR for audit' but lists it in-scope.
severity: minor
resolution: MkdirAll consolidation removed from scope entirely. Plan now lists 'Folding MkdirAll into the WriteFile contract' as out-of-scope. Ticket stays focused on encryption transparency.
status: addressed
---
