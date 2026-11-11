---
id: RR-LQCWWJJ
type: review-response
title: No notice when localStorage is unavailable
finding: Without storage every read is a silent baseline, so the switch shows on but nothing ever notifies.
severity: minor
resolution: The switch probes storage before enabling and shows a settings.notifications.noStorage notice instead. Tests in notificationPrefs and NotificationSetting.
status: addressed
---
