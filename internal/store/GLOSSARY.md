---
zone: restricted
owns:
  Entity header: [entity header]
  Graph query: [graph query]
  Face selection: [face selection]
  Version purge: [version purge, purge selector]
uses: [entity]
avoid: [gated reader, field redactor, acl policy]
---
**Entity header**: an entity row without its body, for collection reads.

**Graph query**: a typed query the backends answer in one store round
trip, including the row gate the ACL compiles.

**Face selection**: which faces of an entity family a read returns.

**Version purge**: the audited, irreversible removal of version rows.

The store stays ungated: who may read what is the ACL's and the visibility
wrappers' business, so the store names no gated reader, field redactor or
ACL policy.
