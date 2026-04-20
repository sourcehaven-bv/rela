---
id: RR-FS0SZ
type: review-response
title: os.UserConfigDir() conflates XDG config/state/cache on Linux, silently changes last_seen_version location from today's ~/.local/state
finding: 'Today''s xdgStateHome uses $XDG_STATE_HOME (~/.local/state on Linux). Plan''s os.UserConfigDir() uses $XDG_CONFIG_HOME (~/.config on Linux). Existing users upgrading from PR #464 have their last_seen_version in the old location; TOFU re-triggers, rollback-protection degrades for one version.'
severity: minor
resolution: 'Accepted: PR #464 is the first release with encryption, not yet in any tagged release, so the practical migration set is zero or near-zero. Plan documents the one-time TOFU reset in release notes. For the new location, use os.UserConfigDir() as the uniform primitive — simpler, consistent across platforms, and semantically ''persistent application data not meant to be synced'' covers all our files. No XDG split across config/state/cache in this PR.'
status: addressed
---
