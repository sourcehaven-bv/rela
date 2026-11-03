---
id: RR-SJ9OQ4
type: review-response
title: Turning a property into a list makes old messages of that type uneditable
finding: 'channelValues.ts:198 carryProp sends a stored scalar back as is for a property the composer does not render. After a config reload turns a string property into list: true, every older message stores a string there. The composer hides the list field, so an edit carries the string, and normalizeList (internal/realtime/validate.go normalizeList) refuses it with ''must be a list of strings''. The user cannot fix this from the UI. Fix: in toEditProps, for a p.list property whose stored value is not an array, leave it out (or wrap a non-empty string as [value]). Add a channelValues test. Mention the config-change case in docs/realtime.md under List properties.'
severity: minor
resolution: 'carryProp now takes the property: for a list property, a stored array goes back as stored, a non-empty stored string goes back as a list of one item, and any other value is left out. Added a channelValues test and a paragraph under List properties in the guide on turning a string property into a list.'
status: addressed
---
