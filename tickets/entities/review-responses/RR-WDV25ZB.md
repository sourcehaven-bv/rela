---
id: RR-WDV25ZB
type: review-response
title: SettingsView uses the global t in a component
finding: The new import used the module t and borrowed footer.settings while the heading stayed a bare string.
severity: significant
resolution: useI18n with a new settings.title key for the heading and the tab; the bare-string count dropped by one.
status: addressed
---
