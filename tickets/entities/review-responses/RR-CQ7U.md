---
id: RR-CQ7U
type: review-response
title: Badge colors are hard-coded hex, not CSS custom properties
finding: 'Badge.vue uses hard-coded hex values in <style> blocks (e.g. color-mix(in srgb, #3b82f6 18%, transparent)) for 6 of 7 badge colors. Only gray uses var(--hover-bg)/var(--muted-text). Runtime palette overrides via document.documentElement.style.setProperty() will NOT affect these hard-coded values. The badges: section in the palette config would be inert without changing Badge.vue to use CSS custom properties.'
severity: significant
resolution: 'Plan updated: add --badge-blue/purple/green/gray/red/orange/yellow CSS custom properties to App.vue. Badge.vue updated to reference variables. Palette badges: section sets these. Default values match current hard-coded hex.'
status: addressed
---
