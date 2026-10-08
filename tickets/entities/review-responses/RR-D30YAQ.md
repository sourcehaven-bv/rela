---
id: RR-D30YAQ
type: review-response
title: '[security] unsafeReadPath compares lexical paths, so symlink and alias paths bypass it'
finding: 'On Linux --ro-bind-try follows a source symlink, so /opt/x -> / or /proc passes the check and mounts the host or host /proc. On macOS Wrap resolves symlinks but the check does not: /private, /private/tmp, /private/var/folders, /private/var/run and case variants like /TMP are accepted, making launchd and ssh-agent listener sockets connectable. Needs operator misconfiguration. Fix: EvalSymlinks (longest existing prefix) and check lexical and resolved paths; resolve os.TempDir() too.'
severity: minor
resolution: 'Same fix as the code review''s symlink finding: lexical and symlink-resolved forms are both checked, protected dirs are resolved too (/private/tmp, /private/etc, /private/var/folders on macOS), case-insensitive on macOS, longest-existing-ancestor resolution for future paths. Tests: TestSetHostReadOnlyChecksSymlinkTargets.'
status: addressed
---
