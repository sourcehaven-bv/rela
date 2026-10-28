---
id: RR-N4OD03
type: review-response
title: TagSelect texts do not follow a language switch
finding: slim-select reads searchText, searchingText and maxValuesMessage once at mount, so an open TagSelect keeps the old language after a switch.
severity: minor
reason: 'Same accepted class as stored toast text in frontend/CLAUDE.md: the next mount picks up the new language, and switching language with a tag picker open is rare. Remounting on locale change would reset the widget mid-edit.'
status: deferred
---
