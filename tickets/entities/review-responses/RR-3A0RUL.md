---
id: RR-3A0RUL
type: review-response
title: 'TZ override: global setting presented as per-field label (mode-error UX)'
finding: 'The clickable ''Times shown in {tz}'' sits under a field but writes uiStore.datetimeTimezone (global, persisted). A user reads it as this field''s tz, clicks to fix one value, and silently re-labels every datetime field app-wide forever - a mode error, and undiscoverable as a setting later. Fix: (1) label states the global scope explicitly (''Display timezone (all times) - {tz}''); (2) the primary control lives in Settings next to theme (the pattern being copied); the inline affordance is a read-only indicator that deep-links there; (3) zone list uses Intl.supportedValuesOf(''timeZone'') with type-ahead + a ''Browser default'' top entry, NOT a hardcoded list; (4) keyboard + screen-reader accessible (''Display timezone, currently Europe/Amsterdam'').'
severity: significant
resolution: 'v1: inline affordance is a READ-ONLY ''Times shown in <effectiveTz>'' indicator (NOT clickable) per user decision. The actual timezone picker lives in Settings (next to theme), using Intl.supportedValuesOf(''timeZone'') with a ''Browser default'' top entry, keyboard + screen-reader accessible. uiStore.datetimeTimezone still holds the global pref; the widget reads effectiveTimezone from it. Clickable inline shortcut deferred to a follow-up. Avoids the mode-error entirely (no inline mutation).'
status: addressed
---
