---
id: RR-XARAI7Q
type: review-response
title: Settings targets gate twice
finding: settingsRelationTargets row-gates in scopedEntities and again in reader.Filter\; a read error now fails the page with 500.
severity: minor
resolution: Comment now says Filter is used for redaction and its row gate drops nothing\; the 500 is deliberate so a failed read is not shown as no choices.
status: addressed
---
