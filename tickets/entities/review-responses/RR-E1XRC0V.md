---
id: RR-E1XRC0V
type: review-response
title: Row ignores descriptor reloads
finding: An entry may name a state or type the held descriptor lacks after a config reload; the page uses checkNames.
severity: minor
resolution: The row calls channelSchema.checkNames on entry change and shows no buttons until loaded (AC 12).
status: addressed
---
