---
id: RR-JIXV9TC
type: review-response
title: Caller options applied twice in inMemoryDefaults
finding: inMemoryDefaults applies the caller options to a scratch struct to read outputDir; a stateful option would fire twice.
severity: minor
reason: Options are pure setters by contract and every existing one is; the scratch pass only reads outputDir.
status: wont-fix
---
