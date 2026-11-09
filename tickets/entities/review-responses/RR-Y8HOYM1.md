---
id: RR-Y8HOYM1
type: review-response
title: --token-secret help and table omit the systemd credential
finding: The secret may come from the systemd credential; help text reads awkwardly with the dotted name.
severity: nit
resolution: Help text and cli-reference name both sources and say per-script overrides are ignored.
status: addressed
---
