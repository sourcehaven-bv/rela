---
owns:
  Gated reader: [visible reader, gated reader, deny reader, script reader]
  Field redactor: [field redactor, policy redactor]
  Unrestricted reader: [unrestricted reader, allow all reader]
uses: [store, tracer, search, acl, entity]
---
**Gated reader**: a read path that row-gates every entity through the
request's ACL before it returns it. A hidden row is nonexistent.

**Field redactor**: removes the property values a `visible:` grant hides
from an entity that passed the row gate.

**Unrestricted reader**: the capability a system job gets at wiring to read
everything; never inferred from identity.
