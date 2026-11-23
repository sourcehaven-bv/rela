---
id: RR-CBV7UU5
type: review-response
title: 'Create form without body: key loses its editor and Duplicate copies a hidden body'
finding: 'Hiding the editor unless body: true is the requested behaviour but Duplicate still submits a body the user cannot see.'
severity: significant
resolution: 'The editor now also shows when the prefill carries content so Duplicate stays visible and clearable. Omitted body: hiding the editor is the requested behaviour and is documented. A body applied by picking a template stays hidden and is saved as before.'
status: addressed
---
