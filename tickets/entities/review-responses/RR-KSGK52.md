---
id: RR-KSGK52
type: review-response
title: User docs say recorded state rather than the chosen version's state
finding: GUIDE-cli-reference, GUIDE-metamodel and GUIDE-postgres-backend said a restore comes back at its recorded state, which is ambiguous for a multi-version history and omitted the guard requirement.
severity: minor
resolution: The three guides now say the chosen version's value, and that a declared transition must enter it with its guard held. Regenerated with just docs.
status: addressed
---
