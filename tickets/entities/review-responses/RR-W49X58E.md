---
id: RR-W49X58E
type: review-response
title: Dead assertion on Object.prototype
finding: 'filters.test.ts: expect(Object.prototype).not.toHaveProperty(''value'') cannot fail, because assigning __proto__ replaces the object''s prototype rather than mutating Object.prototype.'
severity: nit
resolution: Replaced with expect(Object.keys(state)).toEqual(['status']).
status: addressed
---
