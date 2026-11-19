---
id: RR-U915AIM
type: review-response
title: withheld flag doc claims side panel drops it
finding: buildSections is shared with the side panel which never calls omitHiddenSections; the withheld doc said it is always dropped.
severity: minor
resolution: 'The withheld doc now says only the _views path acts on it and that config load refuses permission: on a side-panel section.'
status: addressed
---
