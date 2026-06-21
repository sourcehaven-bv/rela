---
id: RR-BN2MDO
type: review-response
title: fsstore loses an attachment named *.new on restart (temp suffix collision)
finding: 'loadPropertyAttachments skips strings.HasSuffix(name, ''.new''), but ''.new'' is also the live temp-write suffix AND a valid user filename: NormalizeFileName(''report.new'') -> ''report.new'' (only end dots/spaces trimmed) and ValidateFileName accepts it. So a user can upload report.new; it stores/reads fine in-process, but after restart the index loader skips it — file on disk, invisible to the store, stamped property points at a ''missing'' file, cap accounting wrong. multi-cap suffixing can also yield ''x (1).new''. Fix: use a temp marker a normalized name can never equal (leading-dot prefix, e.g. ''.<name>.new-<rand>'') and skip by that prefix, not the user-facing .new suffix.'
severity: significant
resolution: fsstore temp file now uses a leading-dot prefix marker (attachTempPrefix = '.rela-attach-tmp-') instead of the '.new' suffix. A stored attachment name can never start with a dot (store.NormalizeFileName trims leading dots), so the marker is collision-proof; loadPropertyAttachments skips by that prefix. Added storetest FileNameEndingInNewRoundTrips (all backends) + extended fsstore TestPersistence_AttachmentsSurviveReopen to attach a 'notes.new' file and assert it survives a store reopen.
status: addressed
---
