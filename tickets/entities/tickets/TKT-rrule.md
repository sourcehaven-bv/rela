---
effort: s
id: TKT-rrule
kind: enhancement
priority: medium
status: done
title: Custom type regex validations
type: ticket
---

## Description

Extend custom types in metamodel to support regex validations with user-friendly error messages. This enables users to define validated types like RRULE, semver, URLs, etc. without requiring built-in support.

**Feature:**

```yaml
types:
  rrule:
    description: "iCal recurrence rule (RFC 5545)"
    validations:
      - pattern: "^FREQ=(YEARLY|MONTHLY|WEEKLY|DAILY)"
        error: "Must start with valid FREQ"
      - pattern: "^(?!.*COUNT=.*UNTIL=)"
        error: "Cannot specify both COUNT and UNTIL"

  semver:
    validations:
      - pattern: "^[0-9]+\\.[0-9]+\\.[0-9]+"
        error: "Must be valid semver (e.g., 1.2.3)"
```

**Benefits:**
- Multiple simple patterns > one complex regex with opaque errors
- User-defined error messages for clear feedback
- No new built-in types needed - users define what they need
- Reusable across entity types

**Use cases:**
- RRULE validation for recurring schedules
- Semantic version validation
- URL/email format validation
- Custom ID format enforcement
