---
id: RR-5JIN8U
type: review-response
title: Doc wording overstates the literal-'unknown' prohibition (applies to whole-attribution fallback, not the user component)
finding: Migration 0006's comment and the store.Attribution godoc say a literal 'unknown' string would defeat the sweep's fallback and imply it never appears. But withStoreAttribution only skips the wholly-unknown {unknown,unknown} pair; a legitimate CLI principal {User:'unknown', Tool:'cli'} (unset $USER via principal.SystemUser) is forwarded verbatim, so last_edited_by_user='unknown' with tool='cli' CAN appear — intentional and pinned by a test case. Clarify the wording so a future reader doesn't treat a stray 'unknown' user as a bug.
severity: nit
resolution: 'Reworded both spots: migration 0006''s comment and the store.Attribution godoc now distinguish the wholly-unknown fallback (never translated, columns stay NULL) from a partially-unknown principal (stored verbatim, e.g. user=''unknown'' tool=''cli'' under an unset $USER).'
status: addressed
---
