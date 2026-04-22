---
id: RR-F3IA3
type: review-response
title: tmpdir symlink not resolved via realpathSync
finding: os.tmpdir() is a symlink on macOS (/var/folders -> /private/var/folders). Reference fixture does fs.realpathSync(os.tmpdir()); new fixture does not. Path-identity bugs waiting to happen.
severity: significant
resolution: Added TMPDIR = fs.realpathSync(os.tmpdir()) and use it in mkdtempSync.
status: addressed
---
