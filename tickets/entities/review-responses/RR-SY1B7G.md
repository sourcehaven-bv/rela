---
id: RR-SY1B7G
type: review-response
title: 'Design: upgrade path is clamd-only and the failure is silent until first use'
finding: Breaking for every Linux host, not only ClamAV hosts. The upgrade note is only in the attachment-security guide; transforms.md has none and nothing gives the old built-in list as a drop-in value. The startup line for an empty list is Info. /etc/alternatives removal breaks converters reached through Debian alternatives symlinks with a confusing execvp error. Dropping /etc/fonts and /var/lib/fontconfig can silently degrade output of fontconfig-based converters (fallback fonts / empty boxes) rather than fail, so 'run the export once and read the error' does not catch it.
severity: significant
resolution: docs/transforms.md gains an upgrade note giving the previous built-in list as a drop-in RELA_SANDBOX_READ_PATHS value, plus bullets for fontconfig converters (silent font fallback; check output, not exit status) and /etc/alternatives (No such file or directory). New warnIfNoSandboxReadPaths logs a Warn at startup on Linux when a scan or any transform is configured and the list is empty (TestWarnIfNoSandboxReadPaths).
status: addressed
---
