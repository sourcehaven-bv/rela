---
id: RR-RV3O8Z
type: review-response
title: Palette errors now block config load
finding: validatePaletteConfig makes a previously silent bad hex fatal.
severity: minor
reason: 'Intended: the brief requires config validation to reject unknown colours and bad hex. Called out in the PR body; the project keeps no changelog file.'
status: wont-fix
---
