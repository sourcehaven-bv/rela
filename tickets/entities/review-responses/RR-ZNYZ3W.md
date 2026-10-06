---
id: RR-ZNYZ3W
type: review-response
title: Partial same-entity copy refused over the target's own upload
finding: confineFileValues checked every file property of a same-entity copy, so a copy that maps only non-file fields was refused with ErrCopyFileReference whenever the target face held an upload of its own.
severity: significant
resolution: 'confineFileValues now checks only the file properties the copy writes (fields: all or a mapped field). Pinned by TestFaced_PartialCopyKeepsTargetUpload and TestFaced_CopyMappingIntoFilePropertyRefused.'
status: addressed
---
