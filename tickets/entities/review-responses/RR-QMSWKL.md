---
id: RR-QMSWKL
type: review-response
title: Refresh button used the in-button spinner that frontend/CLAUDE.md forbids
finding: Both Refresh buttons swapped their label for a .spinner-sm while loading. frontend/CLAUDE.md states 'No spinners inside buttons' and mandates PendingButton for any button that triggers a request, because a label swap says what is happening, survives prefers-reduced-motion with no fallback, and is its own screen-reader announcement.
severity: significant
resolution: Both Refresh buttons replaced with PendingButton (label 'Refresh', pending-label 'Refreshing…' using U+2026 as required). This also brings the correct anti-flash gate for the explicit-action class (500ms/400ms) and aria-disabled rather than native disabled, so focus is not dropped mid-interaction.
status: addressed
---
