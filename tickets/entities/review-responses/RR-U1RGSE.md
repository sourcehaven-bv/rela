---
id: RR-U1RGSE
type: review-response
title: Rename bulk re-key must leave last_edited_by_* untouched — document that renamed relations keep the content-editor attribution
finding: 'RenameEntity re-keys relations via bulk UPDATE relations SET from_id/to_id, bypassing CreateRelation/UpdateRelation — so it will not stamp attribution, and the sweep cannot see it (re-key does not bump updated_at, TKT-9TQ6I). This is semantically correct (a rename does not edit relation content) but must be stated: the migration''s columns are nullable with no DEFAULT, and the bulk re-key statements must not clobber last_edited_by_*.'
severity: minor
resolution: 'Stays in TKT-ZIRMGM scope: migration adds nullable columns with no DEFAULT; RenameEntity''s bulk re-key statements are verified during implementation to leave last_edited_by_* untouched, and the semantics (renamed relations keep the content-editor attribution) are documented in the postgres-backend guide.'
status: addressed
---
