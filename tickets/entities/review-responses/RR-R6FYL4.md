---
id: RR-R6FYL4
type: review-response
title: Detail actions and commands turn gate errors into 404
finding: resolveDetailActionEntity (actions.go) and the commands entity path mapped a read-gate error to not-found.
severity: minor
resolution: 'The detail-action path now answers a gate error with writeGateError. The commands path keeps the 404 on purpose: it is the plain-text legacy route and its comment documents a gate error as a denial. That fails closed.'
status: addressed
---
