---
id: RR-PKUJ2M
type: review-response
title: where-filter warning logs a user-influenceable entity id unsanitized
finding: 'slog.Warn("gantt: where filter errored; entity excluded", "entity", red.ID, ...) logs a user-influenceable id unsanitized.'
severity: nit
reason: Reviewer's own recommendation. This logging shape is used throughout internal/dataentry, entity ids are prefix-constrained by the metamodel, and the log earns its place — a silent exclusion narrows every ancestor's rolled span, the exact failure the view exists to prevent. Sanitizing this one call site would be inconsistent with the package; if id sanitization in logs is wanted it is a package-wide concern.
status: wont-fix
---
