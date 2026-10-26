---
zone: internal
owns:
  ACL policy: [acl policy]
  Local role: [local role, role relation, role attribution]
  Client ceiling: [scope grant, client baseline, client ceiling]
  Face verdict: [face verdict]
uses: [entity, principal, store]
---
**ACL policy**: the parsed `acl.yaml`: roles, the grants per role, and the
client ceilings, compiled into allowlists when it loads.

**Local role**: a role a principal holds on one entity, conferred by a role
relation to it or an ancestor.

**Client ceiling**: the baseline and scope grants that restrict a client
below the user it acts as.

**Face verdict**: whether the principal may read each face of an entity
family.
