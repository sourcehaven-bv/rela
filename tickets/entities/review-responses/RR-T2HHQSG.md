---
id: RR-T2HHQSG
type: review-response
title: Shown notifications stay after access is revoked
finding: Notifications already shown keep entity titles in the OS notification centre after a refusal.
severity: minor
resolution: 'The notifier keeps shown notifications by tag and closes them, and clears the seen set, when the inbox read is refused. Test: forgets what was seen and closes what it showed (mutation-checked).'
status: addressed
---
