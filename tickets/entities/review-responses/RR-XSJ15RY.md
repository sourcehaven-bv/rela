---
id: RR-XSJ15RY
type: review-response
title: peer key create can point at a file the server does not read
finding: With a systemd credential the server ignores .rela/secrets.yaml; the printed instruction named only the file.
severity: minor
resolution: The stderr text and the docs say to add the line to the systemd credential when the source has one.
status: addressed
---
