---
id: RR-K3WEW2
type: review-response
title: Display-timezone override ignored in list/table columns
finding: 'format.ts formatValue/formatCellValue hardcode browserTimeZone() for the datetime branch; EntityList.vue calls formatCellValue with no tz, so list/table datetime cells always render in the browser zone, ignoring the user''s Settings display-timezone override. The DatetimeWidget uses uiStore.effectiveTimezone, so the FORM honors the override but LISTS do not. This contradicts the shipped copy (SettingsView ''applies to all times'', docs ''applies to every datetime field''). Fix: thread effectiveTimezone into the list path (add a tz param to formatValue/formatCellValue defaulting to browserTimeZone, pass uiStore.effectiveTimezone from EntityList), so the override is honored everywhere.'
severity: significant
resolution: 'Fixed. formatValue and formatCellValue now take an optional tz param (default browserTimeZone()); EntityList.vue passes uiStore.effectiveTimezone so list/table datetime cells honor the Settings display-timezone override, matching the form widget. Regression test added in format.test.ts (formatCellValue with America/New_York asserts 8:30 for a 12:30Z value). Verified: entity detail already uses the widget''s display mode (already zone-aware), so lists were the only gap.'
status: addressed
---
