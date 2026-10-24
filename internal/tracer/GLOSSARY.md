---
gated_by: [visibility]
owns:
  Edge gate: [edge gate]
uses: [store, entity]
avoid: [gated reader, field redactor, acl policy]
---
**Edge gate**: a hook the caller passes to decide which edges a traversal
may follow; the tracer itself decides nothing about access.

The tracer is a pure, ungated reader. The visibility decorator gates it at
wiring, so it names no gated reader, field redactor or ACL policy.
