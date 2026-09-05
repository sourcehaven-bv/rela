---
id: RR-4F65WN
type: review-response
title: WorldBadge entityType is optional, so a forgotten prop renders the raw face name silently
finding: 'WorldBadge.vue now consumes the schema store and takes `entityType?: string`. A call site that omits it gets `{face}` substituted with the coordinate name instead of the label, with no error. Make the prop required so omission is a type error.'
severity: significant
resolution: WorldBadge's entityType prop is required (every call site already passes it), so a forgotten prop is a type error rather than a raw coordinate on screen.
status: addressed
---
