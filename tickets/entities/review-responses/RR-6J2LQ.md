---
id: RR-6J2LQ
type: review-response
title: entity_type empty-string negative test is unreachable
finding: 'Plan''s negative test ''entity_type empty string treated same as undefined'' is unreachable because the YAML schema validation rejects entity_type:"" at startup (docs/data-entry.md:1871-1875: ''entity_type: must be set''). The proposed ternary handles it correctly, but as a test scenario it''s noise. Replace with ''entity_type set to a metamodel type that has no form configured'' — that''s the genuinely interesting branch and isn''t currently distinguished from the missing-entity_type case in the plan.'
severity: minor
resolution: 'Obviated by the redesign: button gating no longer keys off entity_type, so the empty-string negative test is gone. The new validation tests (AC4, AC5) cover the genuinely interesting branches: unknown form ID, empty label, empty form.'
status: addressed
---
