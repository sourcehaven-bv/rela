---
id: RR-RSLIE4F
type: review-response
title: 'Placeholder rendered in copy and conversion templates'
finding: 'copy fields and conversion templates also call Interpolate, where {{current_user.id}} turned from literal text into an empty value.'
resolution: 'An unexported TemplateVars flag enables it for the trigger engine only; a template test pins the literal elsewhere.'
severity: minor
status: addressed
---
