---
id: RR-60LCF1
type: review-response
title: rela rename entity --plural bypasses the new directory-name check
finding: 'internal/cli/rename.go:87 and internal/renametype/renametype.go:86 rename entities/<old> to an unchecked newPlural (default newType+"s"). The schema check only runs on the next load, after the directory has already moved, so the project ends up half-renamed and unloadable. CLI operator only, so not a security issue. Fix: call storage.ValidateDirSegment on newPlural in resolveRenameEntity before any change.'
severity: nit
reason: Pre-existing; filed with the plural-change data loss as TKT-NUMUJD.
status: wont-fix
---
