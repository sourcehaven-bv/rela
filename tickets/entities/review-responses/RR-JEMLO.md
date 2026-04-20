---
id: RR-JEMLO
type: review-response
title: $RELA_USER_STATE_DIR pointing into project tree or synced folder defeats the ticket's entire purpose
finding: Ticket exists to move user-state out of Dropbox/iCloud-synced project trees. If the env-var override lets users point back into them (e.g. $RELA_USER_STATE_DIR=$PWD/.rela-user, or $HOME/Dropbox/rela-state), the escape mechanism is nullified. Edge cases list rejects relative/empty but not the dangerous absolute-into-sync case.
severity: critical
resolution: 'Validate $RELA_USER_STATE_DIR at service construction: (1) must not be inside project root (check via filepath.Rel + strings.HasPrefix with separator); (2) log slog.Warn + echo via out.WriteMessage when resolved path contains well-known sync substrings (~/Dropbox, ~/OneDrive, ~/Library/Mobile Documents, ~/Library/CloudStorage). Warning is non-fatal but visible. Sync-detection is best-effort; document.'
status: addressed
---
