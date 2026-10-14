---
id: RR-YTKVSY
type: review-response
title: Auth credentials allowed over plain http
finding: 'url: http:// with auth sends the secret in cleartext on every poll.'
severity: minor
resolution: Config load refuses auth with a non-https url; test added and documented.
status: addressed
---
