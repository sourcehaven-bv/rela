---
id: RR-HB2VCC
type: review-response
title: placeholder='' was overridden by the default
finding: The placeholder element used `getAttribute('placeholder') || 'Markdown content...'`, so an app
  explicitly asking for no placeholder got the default instead. A native <textarea placeholder=''> shows
  nothing, and placeholder is one of the six contract items.
severity: nit
resolution: Changed to ?? on both the primary and the fallback path, so the default applies only when
  the attribute is absent. Test added.
status: addressed
---
