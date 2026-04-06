---
id: RR-I5IQ
type: review-response
title: Assignment priority order not specified for small palettes
finding: 'With fewer colors than roles (e.g. 8-color palette, 15 roles), semantic and badge roles compete for the same unassigned colors. The plan doesn''t specify priority order. Should be: UI roles first (4), then semantic (4), then badges (7). Within each group, order matters too — should the most constrained roles (specific hue requirements) be assigned before flexible ones.'
severity: significant
resolution: 'Assignment order: UI roles (4) → semantic (4) → badges (7). Most constrained hues assigned first within each group.'
status: addressed
---
