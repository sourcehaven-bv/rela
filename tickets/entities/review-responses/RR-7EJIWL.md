---
id: RR-7EJIWL
type: review-response
title: 'Seeder bypasses entitymanager: unique: and id validation are unenforced'
finding: The seeder writes through store.Store directly. On fs/mem/sqlite the `unique:` rule is enforced only by entitymanager's untransacted scan, so a raw-store seeder can silently create duplicates; ids are validated by storeutil.ValidateID only on some paths. Automations and validations also do not run, so seeded data may be in a state the app never produces (e.g. no checklist entities for in-progress tasks).
severity: significant
resolution: 'Plan changed: the perf schema declares no unique: properties; the generator validates every id with storeutil.ValidateID and generates ids by construction (prefix + counter) so duplicates are impossible; the seeder refuses a non-empty store; automation-derived entities (checklists) are not part of the profile, and the profile is documented as data-only. Writes carry store.WithAttribution for a system:perf-seed principal and an audit record.'
status: addressed
---
