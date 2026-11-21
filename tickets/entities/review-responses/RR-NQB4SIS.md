---
id: RR-NQB4SIS
type: review-response
title: "A dry-run POST per keystroke on a read-only workspace"
finding: "scheduleStagedAffordances kept sending dry-runs that the server always refuses."
severity: minor
resolution: "Addressed: the keystroke dry-run is skipped while the workspace is read-only; the one on mount still runs."
status: addressed
---
