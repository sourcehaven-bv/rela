---
id: AM-filter-url-accepts-schema-names
type: automated-measure
title: 'Test: list filter deep links keep every metamodel-legal name and drop __proto__'
description: Vitest cases in frontend/src/utils/filters.test.ts parse filter keys with names ValidateSchemaName accepts (hyphen, dot, space, digit start, non-ASCII, double quote, constructor, prototype) and drop __proto__ and names with single quotes, backslashes, control characters or outer whitespace.
kind: test
location: frontend/src/utils/filters.test.ts
status: active
---

Vitest cases in `frontend/src/utils/filters.test.ts` parse filter keys whose
names `metamodel.ValidateSchemaName` accepts, and drop `__proto__` and names
with single quotes, backslashes, control characters or outer whitespace.
