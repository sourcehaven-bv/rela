---
id: RR-62HUCX
type: review-response
title: configsql and statesql comments claim to match the store format
finding: 'Both said they match what the store writes: RFC3339Nano, which is no longer true.'
severity: minor
resolution: 'Reworded: those columns are only read back, never compared in SQL, so RFC3339Nano is enough.'
status: addressed
---
