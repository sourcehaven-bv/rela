---
id: RR-19CVEJ
type: review-response
title: 'Design: rela-desktop never applies the sandbox read paths'
finding: The plan claims two composition roots; there are three. cmd/rela-desktop/main.go builds dataentry.NewApp (attachment runner, export engine, document command runners) but never calls cmdexec.SetHostReadOnly (nor SetUnconfinedByDefault). The desktop ships for Linux (.deb/.rpm). Before the change Linux desktops got /etc/fonts, /etc/alternatives, /var/lib/texmf, /var/lib/fontconfig built in; after it they get only the system dirs with no way to add more, so PDF export breaks. On macOS the Homebrew clamd socket is no longer allowed, so scan_cmd rejects every upload. AC2 only tests cmdexec.New, not that every root calls the setter.
severity: critical
resolution: 'Added cmdexec.ApplyHostEnv() (RELA_UNCONFINED_COMMANDS + RELA_SANDBOX_READ_PATHS in one call). rela-desktop main now calls it right after configureLogging, before any LoadProject; the CLI calls it too. This also fixes the pre-existing gap that the desktop never applied SetUnconfinedByDefault. Kept to the environment rather than a desktop hostconfig setting (owner decision: env + server flag); the attachment-security guide documents `launchctl setenv` for a desktop started from the Finder/Dock. TestApplyHostEnv pins the shared wiring.'
status: addressed
---
