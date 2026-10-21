---
id: RR-ILBJ6J
type: review-response
title: Chmod(perm) ignores the umask, so files become more open than today
finding: 'os.OpenFile(tmp, O_CREATE, perm) applies the process umask. f.Chmod(perm) does not. A user with umask 027 or 077 gets 0644 entity, relation, comment and channel files instead of 0640 or 0600. That is a permission regression and contradicts the Security section claim that files never end up more open than before. Fix: do not use CreateTemp plus Chmod. Generate the random suffix yourself and call os.OpenFile(name, O_WRONLY|O_CREATE|O_EXCL, perm) in a small retry loop on fs.ErrExist. This keeps O_EXCL, honours the umask, and needs no chmod. AC2 then becomes: the mode equals perm &^ umask (set syscall.Umask in the test, or compare with a file created by os.WriteFile).'
severity: significant
resolution: 'Plan updated: SafeFS opens the temp file with O_CREATE|O_EXCL and the requested mode, retrying on ErrExist, so the umask applies as today; AC2 checks perm &^ umask.'
status: addressed
---
