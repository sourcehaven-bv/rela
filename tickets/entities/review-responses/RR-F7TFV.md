---
id: RR-F7TFV
type: review-response
title: AC5 sanitizer agreement test didn't exercise filepath.Base
finding: TestAttachmentFilename_RootedFSAgreement asserted against rfs.WriteFile with the raw input, not against filepath.Base(input) — so it was testing RootedFS agreeing with itself, not the production path's sanitizer.
severity: significant
resolution: 'Rewrote the test: now applies filepath.Base to the input before constructing the attachment key. Renamed to TestUploadSanitizerAgreesWithRootedFS. Inputs like ''../../etc/passwd'' now correctly assert ''Base strips → passwd'' which RootedFS accepts. Inputs like ''CON.txt'' (Base preserves) correctly assert RootedFS rejects. Backslash case dropped from table because filepath.Base behaves OS-dependently on backslash-separator input.'
status: addressed
---
