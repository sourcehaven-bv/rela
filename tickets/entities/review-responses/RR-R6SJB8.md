---
id: RR-R6SJB8
type: review-response
title: handleV1Documents left at 134 lines with six exit paths after the split
finding: 'The standalone branch was correctly extracted to its own file, but the entity-anchored branch was left inline and is now the longer of the two: parse, path guards, config lookup, kind check, entity load, read gate, permission gate, type check, render config, refresh, return_to, cache read, render, error branch, response. The plimsoll directive on App is the metric pointing at this function as the next decomposition.'
severity: minor
resolution: Extracted handleV1AnchoredDocument as a sibling of handleV1StandaloneDocument, leaving handleV1Documents as a 22-line dispatcher (down from 134). The two document kinds now read symmetrically, and neither branch can accidentally inherit the other's guards — they differ in their ACL story, not just in whether an id is present. Both are plain functions taking *App, keeping App off its plimsoll load line.
status: addressed
---
