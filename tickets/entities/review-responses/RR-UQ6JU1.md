---
id: RR-UQ6JU1
type: review-response
title: No-op reload causes spurious config_changed
finding: Every reload stores a new Compiled pointer, so in-flight writes get 409 config_changed even when nothing changed.
severity: nit
resolution: Service.record compares the type's shape in the live config with what it recorded instead of config pointers. TestRecord_NoOpReloadDuringWrite.
status: addressed
---
