---
id: RR-R0JLG
type: review-response
title: 'Edge cases under-specified: empty/all-NUL/BOM/conflict-marker collision/EACCES'
finding: 'AC5 covers 8-byte file and partial header. Missing: (a) zero-byte file - current behavior produces empty entity, must be characterized; (b) all-NUL file (corrupt sparse) - not git-crypt, parses to garbage; (c) UTF-8 BOM (3 bytes EF BB BF) - verify splitFrontmatter tolerates; (d) ciphertext that coincidentally contains seven less-than signs - must classify as encrypted, not errConflictedFile. Magic-header check MUST run FIRST, before the conflict-marker scan. Add regression test for that ordering. (e) EACCES (permissions denied) - not git-crypt, distinguish in error message and UI. Add fixtures for each case. Clarify: detection is best-effort; document git-crypt version range checked (v0.6+ uses this header).'
severity: significant
resolution: 'Edge cases now covered in AC11 (unit tests). Header check ordering: AC1 explicitly states detection happens at readDataFile BEFORE parseDocument, so the magic-header check runs before the conflict-marker scan. Test case for ciphertext-containing-conflict-marker added to the table. Empty/zero-byte/all-NUL/BOM cases added. EACCES distinction noted as out of scope (separate concern, not git-crypt).'
status: addressed
---
