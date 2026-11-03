---
id: RR-VYAJYD
type: review-response
title: en/nl toast says "moved on" even when nothing moved
finding: 'frontend/src/i18n/locales/en.json:166, nl.json:166. The SPA also gets session_wrong_state when the operator reloads realtime.yaml with a new in: or when a state is renamed (stranded), and in those cases the session did not move. Neutral wording covers all cases: en "This message type cannot be posted in the session''s current state. Choose another." nl "Dit berichttype kan in de huidige stand van de sessie niet worden geplaatst. Kies een ander." The current nl "past nu niet" is also stiff.'
severity: nit
resolution: 'Toast text is now neutral: en "This message type cannot be posted in the session''s current state. Choose another." and nl "Dit berichttype kan in de huidige stand van de sessie niet worden geplaatst. Kies een ander."'
status: addressed
---
