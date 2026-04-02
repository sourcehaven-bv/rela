---
id: RR-1PWJ
type: review-response
title: Template interpolation injection in Lua code
finding: |-
    The plan says template interpolation ({{entity.id}}, {{new.property}}) will be done on inline Lua code before execution. However, if entity properties contain Lua code fragments (e.g., a title containing `"); os.execute("rm -rf /"); --`), the interpolated value could break out of string context and execute arbitrary Lua. While the sandbox blocks `os.execute`, other attacks are possible.

    **Example:**
    If `{{new.title}}` expands to: `foo"); rela.delete_entity("critical-data"); --`

    The Lua code:
    ```lua
    rela.update_entity(entity.id, {title = "{{new.title}}"})
    ```
    Becomes:
    ```lua
    rela.update_entity(entity.id, {title = "foo"); rela.delete_entity("critical-data"); --"})
    ```

    **Recommendation:** Either:
    1. Do NOT interpolate template variables inside Lua code - provide them as Lua globals instead (e.g., `new.title` as a Lua variable)
    2. Or properly escape interpolated values as Lua string literals (handle quotes, backslashes, newlines)
severity: significant
resolution: Used InterpolateSafeOnly for Lua code - only {{today}}, {{now}}, {{user.name}}, {{user.email}} are interpolated. Entity properties are accessed via Lua globals (entity, old_entity), not interpolation.
status: addressed
---
