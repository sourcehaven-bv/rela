---
id: RR-2FV5XM
type: review-response
title: Default membership relation error does not say it is the default
finding: An operator who never configured membership_relation gets an error about member-of without knowing why that relation is checked.
severity: minor
resolution: When membership_relation is unset the error key adds that member-of is the default and how to set membership_relation.
status: addressed
---
