---
id: RR-MCI945
type: review-response
title: Module size cap does not match the excerpt limit
finding: maxModuleSize was 1 MiB but the envelope excerpts files up to 200 KB, so a module between the two loaded but its errors had no excerpt.
severity: nit
resolution: maxModuleSize now equals maxSourceFileSize and the comment says the read is whole-file and the bound catches mistakes.
status: addressed
---
