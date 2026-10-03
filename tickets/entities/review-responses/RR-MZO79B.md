---
id: RR-MZO79B
type: review-response
title: recordServerSnapshot resets bases of dirty fields
finding: recordServerSnapshot replaces propBase/contentBase/relationsBase without checking pending/queued state. A form reload (transition field, loadView) during a pending edit moves the base to another user's token and the pending save overwrites their value without a 412.
severity: significant
resolution: 'recordServerSnapshot keeps the base of any field with unsaved local state (holdsProp / holdsContent / isRelationsDirty). Test: ''keeps the base of a pending field across a fresh snapshot'' (mutation-verified).'
status: addressed
---
