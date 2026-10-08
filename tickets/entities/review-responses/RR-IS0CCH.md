---
id: RR-IS0CCH
type: review-response
title: Temp-dir guard checks os.TempDir(), not the runner's writable dir
finding: A runner built WithTempDir(dir) writes to dir; a read-only bind over a parent of dir lands after --bind WritableDir and makes it read-only. TMPDIR=/ refuses every path with a misleading reason. Refuse in Wrap any ExtraReadOnly entry that contains spec.WritableDir.
severity: minor
resolution: validateSpec (used by both backends' Wrap) now refuses any ExtraReadOnly entry that contains WritableDir, covering WithTempDir. TestWrapRefusesBindOverWritableDir. The TMPDIR=/ edge refuses every path as containing the temp dir, which is the correct outcome for a temp dir of /.
status: addressed
---
