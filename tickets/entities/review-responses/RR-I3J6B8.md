---
id: RR-I3J6B8
type: review-response
title: Concatenated constant not caught
finding: '''hg'' .. ''h'' produces an invalid value no literal check sees; untested limit.'
severity: minor
resolution: Godoc says concatenation is not folded; test 'concatenated constant' pins the limit.
status: addressed
---
