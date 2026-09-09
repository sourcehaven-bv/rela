---
id: RR-VO8OOQ
type: review-response
title: docs/metamodel.md teaches omitting id_type, which is exactly what the detector flags
finding: 'docs/metamodel.md:479 shows `# id_type: short  # This is the default` as a commented-out line, instructing readers to omit id_type. A schema written that way is precisely what short_id_default.go:47-51 detects, and since FSLoader.Load hard-fails on detection, a reader following the docs produces an unloadable project. Mirrored at docs-project/entities/guides/GUIDE-metamodel.md:485. Fixing the generator while leaving the manual telling people to reintroduce the bug by hand is a half-fix.'
severity: significant
resolution: Uncommented the id_type line in docs/metamodel.md and its docs-project mirror, and replaced the '# This is the default' note with one stating that an entity with no id_type is treated as legacy syntax and refused at load. Also fixed the inverted rename direction at docs/cli-reference.md:1599 (and its mirror), which had 'sequential' → 'auto' where short_id_default.go does 'auto' → 'sequential'.
status: addressed
---
