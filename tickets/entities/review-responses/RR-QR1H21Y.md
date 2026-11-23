---
id: RR-QR1H21Y
type: review-response
title: "About shows no rela build version"
finding: "The About dialog could show the rela version without any configuration."
severity: minor
reason: "The server binary carries no release version, and a build version shown to every user, including portal customers, tells an attacker which fixes are missing. The operator sets app.about.version instead."
status: wont-fix
---
