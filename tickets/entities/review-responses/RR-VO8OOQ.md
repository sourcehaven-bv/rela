---
id: RR-VO8OOQ
type: review-response
title: docs/metamodel.md teaches omitting id_type, which is exactly what the detector flags
finding: 'docs/metamodel.md:479 shows `# id_type: short  # This is the default` as a commented-out line, instructing readers to omit id_type. A schema written that way is precisely what short_id_default.go:47-51 detects, and since FSLoader.Load hard-fails on detection, a reader following the docs produces an unloadable project. Mirrored at docs-project/entities/guides/GUIDE-metamodel.md:485. Fixing the generator while leaving the manual telling people to reintroduce the bug by hand is a half-fix.'
severity: significant
status: open
---
