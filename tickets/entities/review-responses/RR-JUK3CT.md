---
id: RR-JUK3CT
type: review-response
title: Symlinks and special files in the file copy
finding: RootedFS follows symlinks out of the root; FIFOs would block the run; fsstore follows symlinked data files.
severity: minor
resolution: 'Plan updated: os.OpenRoot on both sides, regular files only, special bits dropped, symlinked data files refused.'
status: addressed
---
