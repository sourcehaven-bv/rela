---
id: RR-HTKEE7T
type: review-response
title: type=url input blocks the form save for a stored non-URL value
finding: 'Edit mode used input type=url. Native form validation then refuses to submit a form whose stored value is not a valid URL, such as an existing javascript: or bare-host value, so the whole form could not be saved.'
severity: significant
resolution: 'Edit mode uses type=text with inputmode=url. The e2e spec saves a form holding a javascript: value.'
status: addressed
---
