---
id: RR-R8X7XCT
type: review-response
title: sqlite projects with config only in rela.db have no config on disk
finding: 'storage.md FEAT-UP14BT: config can live only in project_files; opening takes the exclusive lock.'
severity: significant
resolution: 'v1: rela test refuses with a clear message when required config is not on disk (config-in-db project). New AC covers the message. Reading config from the db read-only is part of TKT-DAUS290.'
status: addressed
---
