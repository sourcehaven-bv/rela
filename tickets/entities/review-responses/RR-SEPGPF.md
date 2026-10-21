---
id: RR-SEPGPF
type: review-response
title: cleanupTempFiles deletes another live process's in-flight temp files
finding: 'Not the index cache, but the same cross-process class: New walks entities/ and relations/ and removes every *.tmp, including the PLAN-X.md.tmp that a concurrent rela process is about to rename, so that process''s write fails with a rename error. Worth a follow-up ticket (for example only remove temp files older than a threshold), since the plan is already reviewing concurrent-process behaviour of fsstore''s open path.'
severity: minor
resolution: Filed as follow-up TKT-H63QDR (unique temp names and a grace period in cleanupTempFiles).
status: addressed
---
