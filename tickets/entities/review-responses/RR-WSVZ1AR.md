---
id: RR-WSVZ1AR
type: review-response
title: Native import now takes write services
finding: ImportCmd.Run takes writeServices for every import.
severity: minor
reason: The CLI builds read and write bundles together in newCLIBundles, so the native path costs nothing extra.
status: wont-fix
---
