---
id: RR-X7T1IW
type: review-response
title: CLI would duplicate sqlite wiring
finding: cli may not import the backend packages (arch-lint) and copying openBackend/backendServices would miss future tables silently.
severity: significant
resolution: 'Plan updated: An exported sqlite-tagged appbuild opener is shared by openBackend and the import; a sqlite_master guard test fails on unhandled tables.'
status: addressed
---
