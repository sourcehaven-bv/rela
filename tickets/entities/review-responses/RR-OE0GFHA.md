---
id: RR-OE0GFHA
type: review-response
title: Picker stays hidden when the sidebar load fails
finding: builtinsKnown is set only after a successful sidebar load so a failed load never shows the as-of picker.
severity: minor
reason: Fails toward hidden which is right for a space that hides it. A failed sidebar load already leaves the navigation empty and shows an error; the next load shows the picker.
status: wont-fix
---
